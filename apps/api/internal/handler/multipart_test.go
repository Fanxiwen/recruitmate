package handler

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
)

// multipartBody 测试用 multipart 表单构造器。
type multipartBody struct {
	buf bytes.Buffer
	w   *multipart.Writer
}

// newMultipartBody 创建空表单。
func newMultipartBody() *multipartBody {
	m := &multipartBody{}
	m.w = multipart.NewWriter(&m.buf)
	return m
}

// ensureWriter 惰性初始化 writer。
func (m *multipartBody) ensureWriter() {
	if m.w == nil {
		m.w = multipart.NewWriter(&m.buf)
	}
}

// AddField 添加文本字段。
func (m *multipartBody) AddField(name, value string) {
	m.ensureWriter()
	_ = m.w.WriteField(name, value)
}

// AddFile 添加文件字段（text/plain）。
func (m *multipartBody) AddFile(field, filename string, content []byte) {
	m.ensureWriter()
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+filename+`"`)
	h.Set("Content-Type", "text/plain")
	part, err := m.w.CreatePart(h)
	if err == nil {
		_, _ = part.Write(content)
	}
}

// Close 结束表单。
func (m *multipartBody) Close() {
	m.ensureWriter()
	_ = m.w.Close()
}

// Reader 返回表单内容。
func (m *multipartBody) Reader() *bytes.Reader {
	m.Close()
	return bytes.NewReader(m.buf.Bytes())
}

// ContentType 返回 Content-Type（含 boundary）。
func (m *multipartBody) ContentType() string {
	m.ensureWriter()
	return m.w.FormDataContentType()
}
