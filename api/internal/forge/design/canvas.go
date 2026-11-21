package design

import (
	"encoding/json"
	"errors"
	"html"
	"regexp"
	"strings"
)

// CanvasData is everything needed to redraw (and later revise) a design. It is
// embedded in the published document so the document stays self-contained.
type CanvasData struct {
	Title        string           `json:"title"`
	DesignSystem string           `json:"design_system,omitempty"`
	Theme        string           `json:"theme"`
	Device       string           `json:"device"`
	Width        int              `json:"width"`
	MinH         int              `json:"min_height"`
	Screens      []RenderedScreen `json:"screens"`
}

const dataMarker = `id="forge-canvas-data"`

var dataRe = regexp.MustCompile(`(?s)<script type="application/json" ` + dataMarker + `>(.*?)</script>`)

// ParseCanvas recovers CanvasData from a published design document.
func ParseCanvas(doc []byte) (CanvasData, error) {
	m := dataRe.FindSubmatch(doc)
	if m == nil {
		return CanvasData{}, errors.New("not a canvas design")
	}
	var d CanvasData
	if err := json.Unmarshal(m[1], &d); err != nil || len(d.Screens) == 0 {
		return CanvasData{}, errors.New("unreadable canvas design")
	}
	return d, nil
}

// BuildCanvas renders the self-contained canvas document. Each screen runs in
// its own sandboxed iframe (scripts allowed for Tailwind, but no same-origin
// access), so model-written HTML can never reach the host page.
func BuildCanvas(d CanvasData) ([]byte, error) {
	theme := ThemeByID(d.Theme)
	css := theme.fullCSS()
	if d.DesignSystem != "" {
		css += "\n" + KatalystTokens
	}
	payload := struct {
		CanvasData
		ThemeCSS string `json:"theme_css"`
	}{d, css}
	raw, err := json.Marshal(payload) // escapes <, > and & so it cannot close the script tag
	if err != nil {
		return nil, err
	}
	page := strings.NewReplacer(
		"__TITLE__", html.EscapeString(d.Title),
		"__DATA__", string(raw),
	).Replace(canvasTemplate)
	return []byte(page), nil
}

const canvasTemplate = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="forge:kind" content="design"><meta name="forge:engine" content="canvas">
<title>__TITLE__</title>
<style>
:root{color-scheme:light dark;--bg:#eeeeee;--dot:#c9c9c9;--chip:#ffffff;--chipfg:#111827;--line:#d4d4d8}
@media(prefers-color-scheme:dark){:root{--bg:#242423;--dot:#3a3a39;--chip:#1c1c1b;--chipfg:#f4f4f5;--line:#3f3f46}}
*{box-sizing:border-box}html,body{margin:0;height:100%;overflow:hidden;font-family:ui-sans-serif,system-ui,sans-serif}
body{background:var(--bg);background-image:radial-gradient(var(--dot) 1px,transparent 1px);background-size:22px 22px;color:var(--chipfg)}
#stage{position:absolute;inset:0;cursor:grab;touch-action:none}#stage.drag{cursor:grabbing}
#world{position:absolute;left:0;top:0;transform-origin:0 0;will-change:transform}
.frame{position:absolute;top:0}
.frame h2{margin:0 0 10px 4px;font-size:13px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.frame .shell{background:#fff;border:1px solid var(--line);border-radius:28px;overflow:hidden;box-shadow:0 20px 50px rgba(0,0,0,.18)}
.frame.desktop .shell{border-radius:12px}.frame.tablet .shell{border-radius:22px}
.frame iframe{display:block;width:100%;border:0;background:#fff}
.bar{position:fixed;left:50%;bottom:18px;transform:translateX(-50%);display:flex;align-items:center;gap:4px;padding:6px;border:1px solid var(--line);border-radius:999px;background:var(--chip);box-shadow:0 8px 30px rgba(0,0,0,.18);z-index:5}
.bar button{border:0;background:transparent;color:inherit;font:600 13px ui-sans-serif,system-ui;min-width:34px;height:34px;border-radius:999px;cursor:pointer;padding:0 10px}
.bar button:hover,.bar button:focus-visible{background:rgba(127,127,127,.18);outline:none}.bar span{font:600 12px ui-sans-serif,system-ui;min-width:46px;text-align:center}
.hint{position:fixed;left:16px;top:12px;font:12px ui-sans-serif,system-ui;opacity:.65;z-index:5;pointer-events:none}
</style></head>
<body>
<div class="hint">Scroll to pan · Ctrl/⌘ + scroll or pinch to zoom · drag the canvas to move</div>
<div id="stage"><div id="world"></div></div>
<div class="bar" role="toolbar" aria-label="Canvas controls">
<button id="out" aria-label="Zoom out">−</button><span id="pct" aria-live="polite">100%</span><button id="in" aria-label="Zoom in">+</button><button id="fit" aria-label="Fit to screen">Fit</button>
</div>
<script type="application/json" id="forge-canvas-data">__DATA__</script>
<script>
(function(){
var D=JSON.parse(document.getElementById('forge-canvas-data').textContent);
var GAP=80,TOP=60,world=document.getElementById('world'),stage=document.getElementById('stage');
var fonts='<link rel="preconnect" href="https://fonts.googleapis.com"><link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&family=Playfair+Display:wght@400;700&family=Plus+Jakarta+Sans:wght@300;400;500;600;700;800&family=Space+Grotesk:wght@400;500;700&display=swap" rel="stylesheet">';
function esc(s){return String(s).replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;')}
function wrap(h,id,t){return '<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>'+esc(t)+'</title>'+fonts+
'<script src="https://cdn.tailwindcss.com"><\/script><script src="https://code.iconify.design/iconify-icon/3.0.0/iconify-icon.min.js"><\/script>'+
'<style type="text/tailwindcss">:root{'+D.theme_css+'}*,*::before,*::after{margin:0;padding:0;box-sizing:border-box}html,body{width:100%;min-height:100%}body{font-family:var(--font-sans);background:var(--background);color:var(--foreground);-webkit-font-smoothing:antialiased}*{scrollbar-width:none}*::-webkit-scrollbar{display:none}</style></head><body><div id="root"><div class="relative">'+h+
'</div></div><script>(function(){var f="'+id+'";function s(){var r=document.getElementById("root").firstElementChild;var c=r&&r.firstElementChild;var cls=c&&c.className||"";var h=/h-screen|h-full|min-h-screen/.test(String(cls))?Math.max(innerHeight,'+D.min_height+'):Math.max(document.body.scrollHeight,'+D.min_height+');parent.postMessage({forgeFrame:f,height:h},"*")}setTimeout(s,150);setTimeout(s,700);setTimeout(s,1800)})()<\/script></body></html>'}
var scale=1,tx=0,ty=0,frames={};
var x=40;
D.screens.forEach(function(sc,i){
 var f=document.createElement('div');f.className='frame '+String(D.device||'').toLowerCase();f.style.left=x+'px';f.style.top=TOP+'px';f.style.width=D.width+'px';
 var h=document.createElement('h2');h.textContent=sc.name;
 var shell=document.createElement('div');shell.className='shell';
 var ifr=document.createElement('iframe');ifr.setAttribute('sandbox','allow-scripts');ifr.title=sc.name;ifr.style.height=D.min_height+'px';
 ifr.srcdoc=wrap(sc.html,'f'+i,sc.name);
 shell.appendChild(ifr);f.appendChild(h);f.appendChild(shell);world.appendChild(f);frames['f'+i]=ifr;x+=D.width+GAP;
});
var contentW=x-GAP+40;
addEventListener('message',function(e){var d=e.data;if(d&&d.forgeFrame&&frames[d.forgeFrame]&&typeof d.height==='number'){frames[d.forgeFrame].style.height=Math.min(Math.max(d.height,D.min_height),6000)+'px'}});
function apply(){world.style.transform='translate('+tx+'px,'+ty+'px) scale('+scale+')';document.getElementById('pct').textContent=Math.round(scale*100)+'%'}
function zoomAt(f,cx,cy){var n=Math.min(3,Math.max(.1,scale*f));var k=n/scale;tx=cx-(cx-tx)*k;ty=cy-(cy-ty)*k;scale=n;apply()}
function fit(){var w=stage.clientWidth,hh=stage.clientHeight;var s=Math.min(1,(w-80)/contentW,(hh-140)/(D.min_height+TOP+40));scale=Math.max(.1,s);tx=(w-contentW*scale)/2;ty=24;apply()}
stage.addEventListener('wheel',function(e){e.preventDefault();if(e.ctrlKey||e.metaKey){zoomAt(e.deltaY<0?1.1:1/1.1,e.clientX,e.clientY)}else{tx-=e.deltaX;ty-=e.deltaY;apply()}},{passive:false});
var drag=null;stage.addEventListener('pointerdown',function(e){if(e.target.closest('.shell'))return;drag={x:e.clientX,y:e.clientY,tx:tx,ty:ty};stage.classList.add('drag');stage.setPointerCapture(e.pointerId)});
stage.addEventListener('pointermove',function(e){if(!drag)return;tx=drag.tx+e.clientX-drag.x;ty=drag.ty+e.clientY-drag.y;apply()});
function end(){drag=null;stage.classList.remove('drag')}stage.addEventListener('pointerup',end);stage.addEventListener('pointercancel',end);
function mid(f){zoomAt(f,stage.clientWidth/2,stage.clientHeight/2)}
document.getElementById('in').onclick=function(){mid(1.2)};document.getElementById('out').onclick=function(){mid(1/1.2)};document.getElementById('fit').onclick=fit;
addEventListener('keydown',function(e){if(e.key==='+'||e.key==='=')mid(1.2);if(e.key==='-')mid(1/1.2);if(e.key==='0')fit()});
addEventListener('resize',fit);fit();
})();
</script></body></html>
`
