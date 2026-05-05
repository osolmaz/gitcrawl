export function css() {
  return `
:root{--ink:#0f1115;--text:#1f2328;--text-soft:#3b4147;--muted:#6b7280;--subtle:#9aa1ab;--bg:#fafafa;--paper:#ffffff;--accent:#2563eb;--accent-strong:#1d4ed8;--accent-soft:rgba(37,99,235,.08);--line:#e5e7eb;--line-soft:#eef0f3;--branch:#d0d7de;--code-bg:#0f172a;--code-fg:#e6edf3;--code-border:#1f2937;--code-scroll:#334155;--shadow:rgba(15,17,21,.08);--shadow-strong:rgba(15,17,21,.18);--tag-bg:#ddf4ff;--tag-fg:#0969da;--ring:rgba(37,99,235,.32);color-scheme:light}
[data-theme="dark"]{--ink:#e6edf3;--text:#c9d1d9;--text-soft:#8b949e;--muted:#8b949e;--subtle:#6e7681;--bg:#0d1117;--paper:#161b22;--accent:#58a6ff;--accent-strong:#79b8ff;--accent-soft:rgba(56,139,253,.16);--line:#30363d;--line-soft:#21262d;--branch:#30363d;--code-bg:#010409;--code-fg:#e6edf3;--code-border:#21262d;--code-scroll:#30363d;--shadow:rgba(0,0,0,.5);--shadow-strong:rgba(0,0,0,.7);--tag-bg:rgba(56,139,253,.16);--tag-fg:#58a6ff;--ring:rgba(56,139,253,.4);color-scheme:dark}
*{box-sizing:border-box}
html{scroll-behavior:smooth;scroll-padding-top:24px}
body{margin:0;background:var(--bg);color:var(--text);font-family:"Inter",ui-sans-serif,system-ui,-apple-system,Segoe UI,sans-serif;line-height:1.65;overflow-x:hidden;-webkit-font-smoothing:antialiased;font-feature-settings:"cv02","cv03","cv04","cv11"}
::selection{background:var(--accent);color:#fff}
a{color:var(--accent);text-decoration:none;transition:color .12s}
a:hover{text-decoration:underline;text-underline-offset:.2em}
.shell{display:grid;grid-template-columns:268px minmax(0,1fr);min-height:100vh}
.sidebar{position:sticky;top:0;height:100vh;overflow:auto;padding:24px 22px;background:var(--paper);border-right:1px solid var(--line);scrollbar-width:thin;scrollbar-color:var(--line) transparent}
.sidebar::-webkit-scrollbar{width:6px}
.sidebar::-webkit-scrollbar-thumb{background:var(--line);border-radius:6px}
.sidebar-head{display:flex;align-items:flex-start;justify-content:space-between;gap:10px;margin:0 0 22px}
.brand{display:flex;align-items:center;gap:11px;color:var(--ink);text-decoration:none;flex:1 1 auto;min-width:0}
.brand:hover{text-decoration:none}
.brand img{width:32px;height:32px;border-radius:7px;flex:0 0 auto}
.brand-text{display:block;min-width:0}
.brand strong,.brand-name{display:flex;align-items:center;gap:7px;font-size:1.05rem;line-height:1.1;font-weight:600;letter-spacing:-.005em;color:var(--ink)}
.brand-tag{display:inline-flex;align-items:center;font-family:"JetBrains Mono","SF Mono",ui-monospace,monospace;font-size:.6rem;background:var(--tag-bg);color:var(--tag-fg);padding:2px 6px;border-radius:999px;font-weight:500;letter-spacing:.02em;line-height:1}
.brand-tag::before{content:"";display:inline-block;width:5px;height:5px;border-radius:50%;background:currentColor;margin-right:5px;opacity:.85}
.brand small{display:block;color:var(--muted);font-size:.74rem;margin-top:3px;font-weight:400}
.theme-toggle{appearance:none;background:transparent;border:1px solid var(--line);border-radius:8px;width:34px;height:34px;display:inline-flex;align-items:center;justify-content:center;color:var(--muted);cursor:pointer;flex:0 0 auto;transition:border-color .15s,color .15s,background .15s}
.theme-toggle:hover{border-color:var(--ink);color:var(--ink);background:var(--line-soft)}
.theme-toggle:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
.theme-toggle svg{width:16px;height:16px;display:block}
.theme-icon-sun{display:none}
[data-theme="dark"] .theme-icon-moon{display:none}
[data-theme="dark"] .theme-icon-sun{display:block}
.search{display:block;margin:0 0 22px}
.search span{display:block;color:var(--muted);font-size:.7rem;font-weight:600;text-transform:uppercase;letter-spacing:.08em;margin-bottom:7px}
.search input{width:100%;border:1px solid var(--line);background:var(--bg);border-radius:8px;padding:9px 12px;font:inherit;font-size:.9rem;color:var(--text);outline:none;transition:border-color .15s,box-shadow .15s}
.search input:focus{border-color:var(--accent);box-shadow:0 0 0 3px var(--ring)}
nav section{position:relative;margin:0 0 18px}
nav section::before{content:"";position:absolute;left:6px;top:14px;bottom:6px;width:1.5px;background:var(--branch);border-radius:1px}
nav section:last-child::before{bottom:14px}
nav h2{position:relative;font-size:.68rem;color:var(--muted);text-transform:uppercase;letter-spacing:.09em;margin:0 0 10px;font-weight:600;padding-left:24px;line-height:1.2}
nav h2::before{content:"";position:absolute;left:0;top:50%;width:13px;height:13px;border-radius:50%;background:var(--accent);transform:translateY(-50%);box-shadow:0 0 0 3px var(--paper)}
.nav-link{position:relative;display:block;color:var(--text);text-decoration:none;border-radius:6px;padding:5px 10px 5px 24px;margin:1px 0;font-size:.9rem;line-height:1.4;transition:background .12s,color .12s}
.nav-link::before{content:"";position:absolute;left:3px;top:50%;width:7px;height:7px;border-radius:50%;background:var(--paper);border:1.5px solid var(--branch);transform:translateY(-50%);transition:background .12s,border-color .12s,box-shadow .12s;z-index:1}
.nav-link:hover{background:var(--line-soft);color:var(--ink);text-decoration:none}
.nav-link:hover::before{border-color:var(--muted)}
.nav-link.active{background:var(--accent-soft);color:var(--accent);font-weight:600}
.nav-link.active::before{background:var(--accent);border-color:var(--accent);box-shadow:0 0 0 3px var(--accent-soft)}
main{min-width:0;padding:32px clamp(20px,4.5vw,56px) 80px;max-width:1180px;margin:0 auto;width:100%}
.hero{display:flex;align-items:flex-end;justify-content:space-between;gap:22px;border-bottom:1px solid var(--line);padding:8px 0 22px;margin-bottom:8px;flex-wrap:wrap}
.hero-text{min-width:0;flex:1 1 320px}
.eyebrow{margin:0 0 8px;color:var(--muted);font-weight:600;text-transform:uppercase;letter-spacing:.1em;font-size:.7rem;display:inline-flex;align-items:center;gap:8px}
.eyebrow::before{content:"";display:inline-block;width:8px;height:8px;border-radius:50%;background:var(--accent);box-shadow:0 0 0 2px var(--accent-soft)}
.hero h1{font-size:clamp(1.7rem,3vw,2.4rem);line-height:1.1;letter-spacing:-.018em;margin:0;font-weight:700;color:var(--ink)}
.hero-meta{display:flex;gap:8px;flex:0 0 auto}
.repo,.edit{border:1px solid var(--line);color:var(--text);text-decoration:none;border-radius:7px;padding:6px 11px;font-weight:500;font-size:.83rem;background:var(--paper);transition:border-color .15s,color .15s,background .15s}
.repo:hover,.edit:hover{border-color:var(--ink);color:var(--ink);text-decoration:none}
.edit{color:var(--muted)}
.doc-grid{display:grid;grid-template-columns:minmax(0,1fr);gap:48px;margin-top:24px}
.doc-grid-home{margin-top:8px}
@media(min-width:1180px){.doc-grid{grid-template-columns:minmax(0,72ch) 200px;justify-content:start}.doc-grid-home{grid-template-columns:minmax(0,76ch);justify-content:start}}
.doc{min-width:0;max-width:72ch;overflow-wrap:break-word}
.doc-home{max-width:76ch}
.doc h1{font-size:clamp(2rem,3.6vw,2.8rem);line-height:1.08;letter-spacing:-.02em;margin:0 0 .4em;font-weight:700;color:var(--ink)}
.doc-home h1{font-size:clamp(2.2rem,4.2vw,3.2rem)}
body:not(.home) .doc>h1:first-child{display:none}
.doc h2{font-size:1.45rem;line-height:1.2;margin:2em 0 .5em;font-weight:600;letter-spacing:-.012em;color:var(--ink);position:relative}
.doc h3{font-size:1.1rem;margin:1.7em 0 .35em;position:relative;font-weight:600;color:var(--ink);letter-spacing:-.005em}
.doc h4{font-size:.98rem;margin:1.4em 0 .25em;color:var(--ink);position:relative;font-weight:600}
.doc h2:first-child,.doc h3:first-child,.doc h4:first-child{margin-top:.2em}
.doc :is(h2,h3,h4) .anchor{position:absolute;left:-1.05em;top:0;color:var(--subtle);opacity:0;text-decoration:none;font-weight:400;padding-right:.3em;transition:opacity .12s,color .12s}
.doc :is(h2,h3,h4):hover .anchor{opacity:.7}
.doc :is(h2,h3,h4) .anchor:hover{opacity:1;color:var(--accent);text-decoration:none}
.doc p{margin:0 0 1.05em}
.doc-home>p:first-of-type{font-size:1.12rem;color:var(--text-soft);line-height:1.6;margin:0 0 1.3em;max-width:60ch}
.doc ul,.doc ol{padding-left:1.3rem;margin:0 0 1.15em}
.doc li{margin:.25em 0}
.doc li>p{margin:0 0 .4em}
.doc strong{font-weight:600;color:var(--ink)}
.doc em{font-style:italic}
.doc code{font-family:"JetBrains Mono","SF Mono",ui-monospace,monospace;font-size:.84em;background:var(--line-soft);border:1px solid var(--line);border-radius:5px;padding:.08em .35em;color:var(--ink)}
.doc pre{position:relative;overflow:auto;background:var(--code-bg);color:var(--code-fg);border-radius:8px;padding:14px 18px;margin:1.3em 0;font-size:.85em;line-height:1.6;scrollbar-width:thin;scrollbar-color:var(--code-scroll) transparent;border:1px solid var(--code-border)}
.doc pre::-webkit-scrollbar{height:8px;width:8px}
.doc pre::-webkit-scrollbar-thumb{background:var(--code-scroll);border-radius:8px}
.doc pre code{display:block;background:transparent;border:0;color:inherit;padding:0;font-size:1em;white-space:pre}
.doc pre .copy{position:absolute;top:8px;right:8px;background:rgba(255,255,255,.06);color:var(--code-fg);border:1px solid rgba(255,255,255,.16);border-radius:6px;padding:3px 9px;font:500 .7rem/1 "Inter",sans-serif;cursor:pointer;opacity:0;transition:opacity .15s,background .15s,border-color .15s}
.doc pre:hover .copy,.doc pre .copy:focus{opacity:1}
.doc pre .copy:hover{background:rgba(255,255,255,.12)}
.doc pre .copy.copied{background:var(--accent);border-color:var(--accent);opacity:1}
.doc blockquote{margin:1.4em 0;padding:10px 16px;border-left:3px solid var(--accent);background:var(--accent-soft);border-radius:0 8px 8px 0;color:var(--text)}
.doc blockquote p:last-child{margin-bottom:0}
.doc table{width:100%;border-collapse:collapse;margin:1.2em 0;font-size:.92em}
.doc th,.doc td{border-bottom:1px solid var(--line);padding:9px 10px;text-align:left}
.doc th{font-weight:600;color:var(--ink);background:var(--line-soft);border-bottom:1px solid var(--line)}
.doc hr{border:0;border-top:1px solid var(--line);margin:2.2em 0}
.toc{position:sticky;top:24px;align-self:start;font-size:.84rem;padding-left:14px;border-left:1px solid var(--line);max-height:calc(100vh - 48px);overflow:auto;scrollbar-width:thin;scrollbar-color:var(--line) transparent}
.toc::-webkit-scrollbar{width:5px}
.toc::-webkit-scrollbar-thumb{background:var(--line);border-radius:5px}
.toc h2{font-size:.66rem;color:var(--muted);text-transform:uppercase;letter-spacing:.09em;margin:0 0 10px;font-weight:600}
.toc a{display:block;color:var(--muted);text-decoration:none;padding:4px 0 4px 10px;line-height:1.35;border-left:2px solid transparent;margin-left:-12px;transition:color .12s,border-color .12s}
.toc a:hover{color:var(--ink);text-decoration:none}
.toc a.active{color:var(--accent);border-left-color:var(--accent);font-weight:500}
.toc-l3{padding-left:22px!important;font-size:.94em}
@media(max-width:1179px){.toc{display:none}}
.page-nav{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:48px;border-top:1px solid var(--line);padding-top:20px}
.page-nav>a{display:block;border:1px solid var(--line);background:var(--paper);border-radius:9px;padding:13px 16px;text-decoration:none;color:var(--text);transition:border-color .15s,transform .15s,box-shadow .15s}
.page-nav>a:hover{border-color:var(--accent);text-decoration:none;color:var(--ink)}
.page-nav small{display:block;color:var(--muted);font-size:.7rem;text-transform:uppercase;letter-spacing:.09em;margin-bottom:5px;font-weight:600}
.page-nav span{display:block;font-weight:600;line-height:1.3;color:var(--ink)}
.page-nav-prev{text-align:left}
.page-nav-next{text-align:right;grid-column:2}
.page-nav-prev:only-child{grid-column:1}
.nav-toggle{display:none;position:fixed;top:14px;right:14px;top:calc(14px + env(safe-area-inset-top, 0px));right:calc(14px + env(safe-area-inset-right, 0px));z-index:20;width:40px;height:40px;border-radius:9px;background:var(--paper);border:1px solid var(--line);color:var(--ink);cursor:pointer;padding:10px 9px;flex-direction:column;align-items:stretch;justify-content:space-between;box-shadow:0 4px 14px var(--shadow)}
.nav-toggle span{display:block;width:100%;height:2px;flex:0 0 2px;background:currentColor;border-radius:2px;transition:transform .2s,opacity .2s}
.nav-toggle[aria-expanded="true"] span:nth-child(1){transform:translateY(8px) rotate(45deg)}
.nav-toggle[aria-expanded="true"] span:nth-child(2){opacity:0}
.nav-toggle[aria-expanded="true"] span:nth-child(3){transform:translateY(-8px) rotate(-45deg)}
@media(max-width:900px){
  .shell{display:block}
  .sidebar{position:fixed;inset:0 30% 0 0;max-width:320px;height:100vh;z-index:15;transform:translateX(-100%);transition:transform .25s ease;box-shadow:0 18px 40px var(--shadow-strong);background:var(--paper);pointer-events:none}
  .sidebar.open{transform:translateX(0);pointer-events:auto}
  .nav-toggle{display:flex}
  main{padding:64px 18px 56px}
  .hero{padding-top:6px}
  .hero h1{font-size:clamp(1.5rem,7vw,2rem)}
  .hero-meta{width:100%;justify-content:flex-start}
  .doc{padding:0}
  .doc-grid{margin-top:18px;gap:24px}
  .doc :is(h2,h3,h4) .anchor{display:none}
}
@media(max-width:520px){
  main{padding:60px 14px 48px}
  .doc pre{margin-left:-14px;margin-right:-14px;border-radius:0;border-left:0;border-right:0}
}
`;
}

export function themeInitJs() {
  return `(function(){try{var s=localStorage.getItem('gc-theme');var t=s==='light'||s==='dark'?s:(window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');document.documentElement.dataset.theme=t}catch(e){document.documentElement.dataset.theme='light'}})();`;
}

export function themeToggleHtml() {
  return `<button class="theme-toggle" type="button" aria-label="Toggle color theme" title="Toggle color theme">
        <svg class="theme-icon-moon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
        <svg class="theme-icon-sun" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/></svg>
      </button>`;
}

export function js() {
  return `
const root=document.documentElement;
const themeButton=document.querySelector('.theme-toggle');
const themeMedia=window.matchMedia('(prefers-color-scheme: dark)');
function readStoredTheme(){try{const v=localStorage.getItem('gc-theme');return v==='light'||v==='dark'?v:null}catch(e){return null}}
function writeStoredTheme(t){try{localStorage.setItem('gc-theme',t)}catch(e){}}
function applyTheme(t){root.dataset.theme=t;if(themeButton){const next=t==='dark'?'light':'dark';themeButton.setAttribute('aria-label','Switch to '+next+' theme');themeButton.setAttribute('title','Switch to '+next+' theme')}}
applyTheme(root.dataset.theme==='dark'?'dark':'light');
themeButton?.addEventListener('click',()=>{const next=root.dataset.theme==='dark'?'light':'dark';applyTheme(next);writeStoredTheme(next)});
const onSystemThemeChange=(e)=>{if(!readStoredTheme())applyTheme(e.matches?'dark':'light')};
if(themeMedia.addEventListener)themeMedia.addEventListener('change',onSystemThemeChange);
else themeMedia.addListener?.(onSystemThemeChange);

const sidebar=document.querySelector('.sidebar');
const toggle=document.querySelector('.nav-toggle');
const mobileNav=window.matchMedia('(max-width: 900px)');
const sidebarFocusable='a[href],button,input,select,textarea,[tabindex]';
function setSidebarFocusable(enabled){
  sidebar?.querySelectorAll(sidebarFocusable).forEach((el)=>{
    if(enabled){
      if(el.dataset.sidebarTabindex!==undefined){
        if(el.dataset.sidebarTabindex)el.setAttribute('tabindex',el.dataset.sidebarTabindex);
        else el.removeAttribute('tabindex');
        delete el.dataset.sidebarTabindex;
      }
    }else if(el.dataset.sidebarTabindex===undefined){
      el.dataset.sidebarTabindex=el.getAttribute('tabindex')??'';
      el.setAttribute('tabindex','-1');
    }
  });
}
function setSidebarOpen(open){
  if(!sidebar||!toggle)return;
  sidebar.classList.toggle('open',open);
  toggle.setAttribute('aria-expanded',open?'true':'false');
  if(mobileNav.matches){
    sidebar.inert=!open;
    if(open)sidebar.removeAttribute('aria-hidden');
    else sidebar.setAttribute('aria-hidden','true');
    setSidebarFocusable(open);
  }else{
    sidebar.inert=false;
    sidebar.removeAttribute('aria-hidden');
    setSidebarFocusable(true);
  }
}
setSidebarOpen(false);
toggle?.addEventListener('click',()=>setSidebarOpen(!sidebar?.classList.contains('open')));
document.addEventListener('click',(e)=>{if(!sidebar?.classList.contains('open'))return;if(sidebar.contains(e.target)||toggle?.contains(e.target))return;setSidebarOpen(false)});
document.addEventListener('keydown',(e)=>{if(e.key==='Escape')setSidebarOpen(false)});
const syncSidebarForViewport=()=>setSidebarOpen(sidebar?.classList.contains('open')??false);
if(mobileNav.addEventListener)mobileNav.addEventListener('change',syncSidebarForViewport);
else mobileNav.addListener?.(syncSidebarForViewport);
const input=document.getElementById('doc-search');
input?.addEventListener('input',()=>{const q=input.value.trim().toLowerCase();document.querySelectorAll('nav section').forEach(sec=>{let any=false;sec.querySelectorAll('.nav-link').forEach(a=>{const m=!q||a.textContent.toLowerCase().includes(q);a.style.display=m?'block':'none';if(m)any=true});sec.style.display=any?'block':'none'})});
document.querySelectorAll('.doc pre').forEach(pre=>{const btn=document.createElement('button');btn.type='button';btn.className='copy';btn.textContent='Copy';btn.addEventListener('click',async()=>{const code=pre.querySelector('code')?.textContent??'';try{await navigator.clipboard.writeText(code);btn.textContent='Copied';btn.classList.add('copied');setTimeout(()=>{btn.textContent='Copy';btn.classList.remove('copied')},1400)}catch{btn.textContent='Failed';setTimeout(()=>{btn.textContent='Copy'},1400)}});pre.appendChild(btn)});
const tocLinks=document.querySelectorAll('.toc a');
if(tocLinks.length){const map=new Map();tocLinks.forEach(a=>{const id=a.getAttribute('href').slice(1);const el=document.getElementById(id);if(el)map.set(el,a)});const setActive=l=>{tocLinks.forEach(x=>x.classList.remove('active'));l.classList.add('active')};const obs=new IntersectionObserver(entries=>{const visible=entries.filter(e=>e.isIntersecting).sort((a,b)=>a.boundingClientRect.top-b.boundingClientRect.top);if(visible.length){const link=map.get(visible[0].target);if(link)setActive(link)}},{rootMargin:'-15% 0px -65% 0px',threshold:0});map.forEach((_,el)=>obs.observe(el))}
`;
}

export function faviconSvg() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="gitcrawl">
<rect width="64" height="64" rx="12" fill="#0f1115"/>
<path d="M20 22v20M22 20c5 2 10 4 22 11M22 44c5-2 10-4 22-11" stroke="#58a6ff" stroke-width="2.6" stroke-linecap="round" fill="none"/>
<circle cx="20" cy="20" r="4.4" fill="#58a6ff"/>
<circle cx="20" cy="44" r="4.4" fill="#58a6ff"/>
<circle cx="44" cy="32" r="4.4" fill="#2dd4bf"/>
</svg>`;
}
