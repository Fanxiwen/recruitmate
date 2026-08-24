/** 页脚：机构全称 + 双语品牌语 */
export function Footer() {
  return (
    <footer className="border-t border-slate-100 bg-white">
      <div className="mx-auto max-w-5xl px-4 py-10 text-center">
        <p className="text-sm font-semibold text-brand-800">
          中国-葡语（西语）国家经济贸易服务中心
        </p>
        <p className="mt-1 text-xs tracking-wide text-slate-400">
          Centro de Serviços Económicos e Comerciais China-Países de Língua Portuguesa (e Espanhola)
        </p>
        <div className="mx-auto mt-4 h-px w-24 bg-linear-to-r from-transparent via-gold-400 to-transparent" />
        <p className="mt-4 text-sm text-slate-600">
          连接中国与葡语（西语）国家的「一站式」综合服务平台
        </p>
        <p className="mt-2 text-xs text-slate-500">
          © {new Date().getFullYear()} 中葡经贸中心 · 人才招聘
        </p>
      </div>
    </footer>
  );
}
