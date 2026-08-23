// Package file 封装文件存储（MinIO，S3 兼容）与简历文本提取（PDF/DOCX/TXT）。
package file

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// Storage 对象存储接口。
type Storage interface {
	EnsureBucket(ctx context.Context) error
	// Upload 上传对象，返回对象 key。
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// PresignedGetURL 生成预签名 GET 下载地址。
	PresignedGetURL(ctx context.Context, key string, expire time.Duration) (string, error)
	// Download 下载对象全部内容（用于 worker 文本提取）。
	Download(ctx context.Context, key string) ([]byte, error)
}

// Config MinIO 配置。
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// MinioStorage 基于 minio-go 的实现。
type MinioStorage struct {
	client *minio.Client
	bucket string
}

// NewMinioStorage 创建 MinIO 客户端。
func NewMinioStorage(cfg Config) (*MinioStorage, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: new client: %w", err)
	}
	return &MinioStorage{client: mc, bucket: cfg.Bucket}, nil
}

// EnsureBucket 确保桶存在。
func (s *MinioStorage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("minio: bucket exists: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("minio: make bucket: %w", err)
		}
	}
	return nil
}

// Upload 上传对象。
func (s *MinioStorage) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("minio: put object %s: %w", key, err)
	}
	return nil
}

// PresignedGetURL 生成预签名下载 URL。
func (s *MinioStorage) PresignedGetURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expire, url.Values{})
	if err != nil {
		return "", fmt.Errorf("minio: presign %s: %w", key, err)
	}
	return u.String(), nil
}

// Download 下载对象内容。
func (s *MinioStorage) Download(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio: get object %s: %w", key, err)
	}
	defer obj.Close()
	data, err := io.ReadAll(io.LimitReader(obj, 16<<20)) // 上限 16MB
	if err != nil {
		return nil, fmt.Errorf("minio: read object %s: %w", key, err)
	}
	return data, nil
}

// ============ 文本提取 ============

// MaxResumeSize 简历文件大小上限（5MB）。
const MaxResumeSize = 5 << 20

// AllowedResumeExts 简历文件扩展名白名单。
var AllowedResumeExts = map[string]bool{
	".pdf":  true,
	".docx": true,
	".txt":  true,
}

// IsAllowedExt 检查扩展名是否在白名单内。
func IsAllowedExt(filename string) bool {
	return AllowedResumeExts[strings.ToLower(filepath.Ext(filename))]
}

// ContentTypeFor 根据扩展名返回 MIME 类型。
func ContentTypeFor(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "text/plain"
	}
}

// ExtractText 从文件内容提取纯文本（PDF 用 pdfcpu、DOCX 用 zip+xml、TXT 直读）。
func ExtractText(filename string, data []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return extractPDFText(data)
	case ".docx":
		return extractDOCXText(data)
	case ".txt":
		return string(data), nil
	default:
		return "", fmt.Errorf("unsupported file type: %s", filepath.Ext(filename))
	}
}

// extractPDFText 使用 pdfcpu 提取 PDF 文本。
func extractPDFText(data []byte) (string, error) {
	var buf bytes.Buffer
	rs := bytes.NewReader(data)
	err := api.ExtractContent(rs, nil, func(r io.Reader, _ int) error {
		_, err := io.Copy(&buf, r)
		return err
	}, nil)
	if err != nil {
		return "", fmt.Errorf("pdf extract: %w", err)
	}
	return buf.String(), nil
}

// extractDOCXText 解析 docx（zip）中 word/document.xml 的 <w:t> 文本。
func extractDOCXText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("docx open zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("docx open %s: %w", f.Name, err)
		}
		defer rc.Close()
		body, err := io.ReadAll(io.LimitReader(rc, 32<<20))
		if err != nil {
			return "", fmt.Errorf("docx read %s: %w", f.Name, err)
		}
		return xmlText(body), nil
	}
	return "", fmt.Errorf("docx: word/document.xml not found")
}

// xmlText 从 XML 中抽取 <w:t> 与 <w:p> 标签内的文本（段落换行）。
func xmlText(xml []byte) string {
	var sb strings.Builder
	// 简单状态机：识别 <w:t ...>text</w:t> 与 </w:p>
	i := 0
	n := len(xml)
	for i < n {
		if xml[i] != '<' {
			i++
			continue
		}
		end := bytes.IndexByte(xml[i:], '>')
		if end < 0 {
			break
		}
		tag := string(xml[i+1 : i+end])
		if strings.HasPrefix(tag, "w:t") {
			// 找到闭合标签
			closeIdx := bytes.Index(xml[i+end+1:], []byte("</w:t>"))
			if closeIdx < 0 {
				break
			}
			sb.Write(xml[i+end+1 : i+end+1+closeIdx])
			i += end + 1 + closeIdx + len("</w:t>")
			continue
		}
		if tag == "/w:p" {
			sb.WriteString("\n")
		}
		i += end + 1
	}
	return sb.String()
}
