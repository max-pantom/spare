package webui

const Styles = `
:root{color-scheme:dark;font-family:Inter,ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;font-synthesis:none;-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale;--bg:#1c1c1c;--surface:#242424;--surface-2:#2b2b2b;--text:#f5f5f5;--muted:#a9a9a9;--line:#ffffff12;--good:#52d681;--bad:#ff756b}
*{box-sizing:border-box}body{margin:0;min-height:100vh;background:var(--bg);color:var(--text);font-size:14px;font-weight:400;line-height:1.4}
a{color:inherit;text-underline-position:from-font;text-decoration-thickness:from-font}.shell{width:min(100% - 24px,1080px);margin-inline:auto;padding-block:20px 32px}
header{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;margin-bottom:16px}.eyebrow{margin:0 0 4px;color:var(--muted);font-size:11px;font-weight:400;letter-spacing:.06em;text-transform:uppercase}
h1{margin:0;font-size:20px;font-weight:500;line-height:1.2;letter-spacing:-.02em;text-wrap:balance}h2{margin:0;font-size:14px;font-weight:500;line-height:1.3}p{margin:0;color:var(--muted);font-weight:400;text-wrap:pretty}
.subtitle{max-width:65ch;margin-top:6px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,280px),1fr));gap:10px}.card{padding:12px;border-radius:10px;background:linear-gradient(180deg,#2b2b2bb5,#202020b5);box-shadow:inset 0 .1px .2px .1px #ffffff4d,0 0 0 1px #ffffff08}
.stack{display:grid;gap:12px}.row{display:flex;align-items:center;justify-content:space-between;gap:16px}.meta{font-size:12px;color:var(--muted);font-variant-numeric:tabular-nums}.status{display:inline-flex;align-items:center;gap:6px}.status::before{content:"";width:6px;height:6px;border-radius:50%;background:currentColor}.status.good{color:var(--good)}.status.bad{color:var(--bad)}
form{display:grid;gap:10px}.field{display:grid;gap:6px}label{font-weight:400}input,select,textarea{width:100%;min-height:40px;border:0;border-radius:8px;padding:9px 10px;background:#ffffff0b;color:var(--text);box-shadow:inset 0 0 0 1px var(--line);font:inherit}textarea{min-height:96px;resize:vertical}
button,.button{display:inline-flex;align-items:center;justify-content:center;min-height:40px;border:0;border-radius:9px;padding:0 13px;background:#f1f1f1;color:#191919;font:500 14px Inter,system-ui,sans-serif;text-decoration:none;cursor:pointer}.button.secondary,button.secondary{background:#ffffff0d;color:var(--text);box-shadow:inset 0 0 0 1px var(--line)}
button:active,.button:active{transform:scale(.96)}button:focus-visible,.button:focus-visible,input:focus-visible,select:focus-visible,textarea:focus-visible,a:focus-visible{outline:2px solid #fff;outline-offset:2px}.actions{display:flex;align-items:center;flex-wrap:wrap;gap:8px}.empty{padding:36px;text-align:center}.danger{color:#ffb4ad}
table{width:100%;border-collapse:collapse}th,td{text-align:start;padding:12px 8px;border-bottom:1px solid var(--line);vertical-align:top}th{color:var(--muted);font-size:12px;font-weight:500}.progress{height:6px;overflow:hidden;border-radius:99px;background:#ffffff0c}.progress>span{display:block;height:100%;background:#d8d8d8}
@media(max-width:640px){.shell{width:min(100% - 20px,1080px);padding-block:16px 28px}header{display:grid}.row{align-items:flex-start}h1{font-size:18px}input,select,textarea{font-size:16px}}
@media(prefers-reduced-motion:no-preference){button,.button{transition:transform 120ms ease-out,background-color 120ms ease-out}}
@media(forced-colors:active){button,.button,input,select,textarea{border:1px solid ButtonText}}
`
