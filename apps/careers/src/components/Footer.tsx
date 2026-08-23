/** 页脚：简洁版权信息 */
export function Footer() {
  return (
    <footer className="border-t border-slate-100 bg-white">
      <div className="mx-auto max-w-5xl px-4 py-8 text-center">
        <p className="text-sm text-slate-400">
          © {new Date().getFullYear()} Recruitmate · 示例科技
        </p>
        <p className="mt-1 text-xs text-slate-300">与优秀的人，做有价值的事</p>
      </div>
    </footer>
  );
}
