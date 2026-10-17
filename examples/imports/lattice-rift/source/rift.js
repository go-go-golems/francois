/* rift.js — the unhinged arcade: stage.js's section compositor, then four verbs applied to the whole page.

   Per frame:
     1. dshost runs each visible rift's frame(h): its two DS screens into textures;
     2. each visible section's background (glsl.bg) and its device are drawn and seamed together → SCN;
     3. STRANDS: a half-resolution field (direction, density, hue) advected along itself and curl noise;
        a pointer drag anywhere on the page draws into it;
     4. MELT: SCN, torn by up to two seams (arrows / gamepad) with hyperreality in the gaps, pulled along
        the strands, then melted wherever keys have put heat: hot pixels keep sliding down a feedback buffer,
        and as the heat dies the picture reassembles in the wrong places first;
     5. FINAL: microglobular molecules (space), the strands' glow, colour split and grain → canvas.
   The DOM copy rides along: panels follow their half of a tear, sag where it is hot, blur into molecules. */
(function () {
'use strict';
const DS = window.DS, RIFTS = window.RIFTS || [];
const EDGES = { straight: 0, wave: 1, tear: 2, pixel: 3, glitch: 4, zigzag: 5 };
const MARGIN = 80;

/* textures a rift builds itself (same helpers as games-b.js, kept here so the page needs only these files) */
window.GAMEKIT = window.GAMEKIT || {
  atlas(h, key, cells) {
    const T = h.tex.get(key); if (T && T.sig) return T.tex;
    const rows = Math.ceil(cells.length / 8), c = document.createElement('canvas'); c.width = 1024; c.height = 128 * rows;
    const x = c.getContext('2d'); x.fillStyle = '#000'; x.fillRect(0, 0, c.width, c.height);
    x.textAlign = 'center'; x.textBaseline = 'middle'; x.globalCompositeOperation = 'lighter';
    cells.forEach(([t, font, px, weight], i) => {
      const cx = (i % 8) * 128 + 64, cy = Math.floor(i / 8) * 128 + 66; x.font = `${weight || 400} ${px}px ${font}`;
      for (const [col, blur] of [['#00ff00', 4], ['#0000ff', px * 0.22]]) { x.save(); x.shadowColor = col; x.shadowBlur = blur; x.shadowOffsetX = 20000; x.fillStyle = col; x.fillText(t, cx - 20000, cy); x.restore(); }
      x.fillStyle = '#ff0000'; x.fillText(t, cx, cy);
    });
    const tex = h.H.texFromCanvas(c), gl = h.H.gl;
    gl.bindTexture(gl.TEXTURE_2D, tex); gl.generateMipmap(gl.TEXTURE_2D); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);   // small glyphs average instead of aliasing away
    h.tex.set(key, { sig: 'atlas', tex }); return tex;
  },
  measure(text, font, px, weight = 400) { const x = (this._m || (this._m = document.createElement('canvas').getContext('2d'))); x.font = `${weight} ${px * 2}px ${font}`; return x.measureText(text).width / 2; }
};

/* ---------- GLSL: the section contract (as stage.js) ---------- */
const STAGE_HEAD = `
uniform vec2  iStage;    // the viewport, in stage px
uniform float u_k;       // stage px per CSS px
uniform vec4  u_band;    // this section's box: x, y (bottom edge, y-up), w, h, stage px
uniform float iScroll;   // 0 as the section's top enters at the bottom of the window, 1 as its bottom leaves at the top
uniform vec4  iMouse;    // the mouse, stage px y-up; z 1 while a button is down
uniform vec4  u_rTop, u_rBot, u_rTitle;
uniform sampler2D u_scrTop, u_scrBot, u_title;
uniform float u_chaos;   // how unhinged the page is right now, 0..~3
vec2 view(vec2 f){ return (f - .5*iStage)/iStage.y; }
vec2 sect(vec2 f){ return (f - u_band.xy - .5*u_band.zw)/iStage.y; }
vec2 mouseView(){ return view(iMouse.xy); }
vec3 titleAt(vec2 f){ vec2 uv = (f - u_rTitle.xy)/u_rTitle.zw; if (any(lessThan(uv, vec2(0.))) || any(greaterThan(uv, vec2(1.)))) return vec3(0.); return texture(u_title, uv).rgb; }
vec2 titleUV(vec2 f){ return (f - u_rTitle.xy)/u_rTitle.zw; }
vec3 ns_glow(sampler2D t, vec4 R, vec2 f){ vec2 h = .5*R.zw, c = R.xy + h; float d = length(max(abs(f - c) - h, 0.));
  return textureLod(t, clamp((f - R.xy)/R.zw, 0., 1.), 5.).rgb*exp(-d/(.5*R.z)); }
vec3 screenLight(vec2 f){ return ns_glow(u_scrTop, u_rTop, f) + ns_glow(u_scrBot, u_rBot, f); }
// the top / bottom screen as a material: uv 0..1 (y-up); mirrored outside, so it tiles without seams
vec3 topTex(vec2 uv){ return texture(u_scrTop, 1. - abs(1. - mod(uv, 2.))).rgb; }
vec3 botTex(vec2 uv){ return texture(u_scrBot, 1. - abs(1. - mod(uv, 2.))).rgb; }
vec2 devCentre(){ return .5*(u_rTop.xy + .5*u_rTop.zw + u_rBot.xy + .5*u_rBot.zw); }
mat2 rot(float a){ float c = cos(a), s = sin(a); return mat2(c, -s, s, c); }
`;
const DEVICE = `
uniform vec4 u_rTop, u_rBot; uniform sampler2D u_scrTop, u_scrBot; uniform float u_k; uniform vec3 u_shell;
float sdBox(vec2 p, vec2 b, float r){ vec2 q = abs(p) - b + r; return length(max(q, 0.)) + min(max(q.x, q.y), 0.) - r; }
vec3 lcd(sampler2D t, vec4 R, vec2 f){
  vec2 uv = (f - R.xy)/R.zw; ivec2 s = textureSize(t, 0); vec2 px = uv*vec2(s);
  vec3 c = texelFetch(t, clamp(ivec2(px), ivec2(0), s - 1), 0).rgb;
  float cell = R.z/float(s.x); vec2 g = abs(fract(px) - .5);
  c *= 1. - .14*smoothstep(2.5, 5., cell)*smoothstep(.5 - 1.2/cell, .5, max(g.x, g.y));
  float band = uv.y - .55*uv.x;
  return c + .05*smoothstep(.15, .35, band)*smoothstep(.7, .4, band);
}
void mainImage(out vec4 o, in vec2 f){
  float k = u_k, bz = 9.*k, rr = 14.*k;
  vec2 hT = .5*u_rTop.zw, hB = .5*u_rBot.zw, cT = u_rTop.xy + hT, cB = u_rBot.xy + hB;
  float dT = sdBox(f - cT, hT + bz, rr), dB = sdBox(f - cB, hB + bz, rr);
  float g0 = u_rBot.y + u_rBot.w + bz, g1 = u_rTop.y - bz, hh = max(.5*(g1 - g0), 1.);
  vec2 hc = vec2(cT.x, .5*(g0 + g1));
  float dH = sdBox(f - hc, vec2(hT.x*.84, hh + 3.*k), hh*.9);
  float d = min(min(dT, dB), dH), a = 1. - smoothstep(-.75, .75, d);
  vec2 sh = vec2(0., 24.*k);
  float shadow = .55*(1. - smoothstep(-14.*k, 46.*k, min(sdBox(f + sh - cT, hT + bz, rr), sdBox(f + sh - cB, hB + bz, rr))));
  float lit = clamp((f.y - u_rBot.y)/(u_rTop.y + u_rTop.w - u_rBot.y), 0., 1.);
  vec3 col = u_shell*(.8 + .35*lit) + .12*smoothstep(-2.5*k, -.5*k, d)*step(d, 0.);
  col += .06*(.5 + .5*cos(6.283*(f.x*.002/k + f.y*.001/k + vec3(0., .33, .67))));   // a little oil on the plastic
  if (dH < 0. && dT > 0. && dB > 0.) { float n = (f.y - hc.y)/hh; col = u_shell*(.45 + .8*(1. - n*n));
    col = mix(col, vec3(1., .2, .6), 1. - smoothstep(-.6, .6, length(f - vec2(cT.x + hT.x*.7, hc.y)) - 2.5*k)); }
  float sT = sdBox(f - cT, hT, 2.*k), sB = sdBox(f - cB, hB, 2.*k), s = min(sT, sB);
  col *= 1. - .5*exp(-max(s, 0.)/(1.6*k));
  if (sT < 1.) col = mix(lcd(u_scrTop, u_rTop, f), col, smoothstep(-.5, .5, sT));
  if (sB < 1.) col = mix(lcd(u_scrBot, u_rBot, f), col, smoothstep(-.5, .5, sB));
  o = vec4(col*a, a + shadow*(1. - a));
}`;
const SEAM = `
uniform sampler2D u_sec; uniform float u_edge, u_first, u_k; uniform int u_kind; uniform vec3 u_glow;
void mainImage(out vec4 o, in vec2 f){
  vec3 c = texelFetch(u_sec, ivec2(f), 0).rgb;
  if (u_first > .5) { o = vec4(c, 1.); return; }
  float x = f.x/u_k, t = iTime, off = 0.;
  if (u_kind == 1) off = 26.*sin(x*.014 + t*1.7) + 11.*sin(x*.051 - t*2.6);
  else if (u_kind == 2) off = 60.*(fbm(vec2(x*.011, 3. + t*.1)) - .5) + 5.*(hash12(vec2(floor(x/2.), 7.)) - .5);
  else if (u_kind == 3) off = 22.*(floor(hash12(vec2(floor(x/24.), floor(t*5.)))*5.) - 2.);
  else if (u_kind == 4) { float r = hash12(vec2(floor(x/70.), floor(t*9.))); off = r > .6 ? (r - .8)*260. : 0.; }
  else if (u_kind == 5) off = 60.*abs(fract(x/60. + t*.1) - .5) - 15.;
  float d = (u_edge + off*u_k - f.y)/u_k, a = smoothstep(-.6, .6, d);
  o = vec4(c*a + u_glow*(1.2*exp(-abs(d)/1.5) + .4*exp(-abs(d)/18.)), a);
}`;

/* ---------- GLSL: the verbs ---------- */
const STRANDS = `
uniform sampler2D u_prev; uniform vec4 u_seg; uniform float u_down, u_hue, u_k2, u_clear; uniform vec2 u_size;
vec2 curl(vec2 p){ float e = .12; return vec2(vnoise(p + vec2(0., e)) - vnoise(p - vec2(0., e)), vnoise(p - vec2(e, 0.)) - vnoise(p + vec2(e, 0.)))/(2.*e); }
float sdSeg(vec2 p, vec2 a, vec2 b){ vec2 pa = p - a, ba = b - a; return length(pa - ba*clamp(dot(pa, ba)/max(dot(ba, ba), 1e-4), 0., 1.)); }
void mainImage(out vec4 o, in vec2 f){
  if (u_clear > .5) { o = vec4(.5, .5, 0., 0.); return; }
  vec2 v0 = texture(u_prev, f/u_size).xy*2. - 1.;
  vec2 c = curl(f/u_size.y*3.5 + iTime*.06);
  vec2 back = f - (v0*1.3 + c*1.1)*u_k2;              // each strand creeps along itself, and the noise bends it
  vec4 s = texture(u_prev, back/u_size);
  vec2 v = normalize(mix(s.xy*2. - 1., normalize(c + 1e-4), .025) + 1e-4);
  float dens = s.z*.998 - .0003, hue = s.w;
  float side = texture(u_prev, (back + vec2(-v.y, v.x)*2.*u_k2*sin(iTime*3. + f.x*.05))/u_size).z;   // strands fray
  dens = max(dens, side*.985*step(.9, vnoise(f*.08 + iTime)));
  if (u_down > .5) {
    float d = sdSeg(f, u_seg.xy, u_seg.zw), m = exp(-d*d/(10.*u_k2*u_k2));
    vec2 dir = normalize(u_seg.zw - u_seg.xy + 1e-4);
    dens = max(dens, m); v = normalize(mix(v, dir, m) + 1e-4); hue = mix(hue, u_hue, m);
  }
  o = vec4(v*.5 + .5, clamp(dens, 0., 1.), hue);
}`;
const MELT = `
uniform sampler2D u_scn, u_prev, u_str;
uniform vec4 u_spl[16];          // heat: x, y (stage px), start time, strength
uniform vec4 u_tearV, u_tearH;   // seam centre xy, opening (stage px), seed
uniform float u_now, u_k;
uniform vec2 u_size;
float heat(vec2 f){ float h = 0.;
  for (int i = 0; i < 16; i++) { vec4 s = u_spl[i]; if (s.w <= 0.) continue; float age = u_now - s.z; vec2 d = (f - s.xy)/(u_size.y*.2);
    h += s.w*exp(-dot(d, d))*exp(-age*.3); }
  return h; }
// what is under the page: the page itself, folded through a kaleidoscope and a foam of circle inversions
vec3 hyper(vec2 f){
  vec2 p = (f - .5*u_size)/u_size.y*2.4; float t = u_now*.13;
  float a = atan(p.y, p.x), r = length(p), k = 2.*PI/7.; a = abs(mod(a + t, k) - k*.5); p = r*vec2(cos(a), sin(a));
  float trap = 1e9;
  for (int i = 0; i < 7; i++) { p = abs(p) - vec2(.55, .32); float m = clamp(.75/dot(p, p), 1., 3.5); p *= m; trap = min(trap, abs(length(p) - .7)); p = mat2(cos(t), -sin(t), sin(t), cos(t))*p; }
  vec3 c = .5 + .5*cos(6.283*(trap*1.4 + vec3(0., .33, .67) + u_now*.07));
  vec3 g = texture(u_scn, 1. - abs(1. - mod(p*.04 + .5, 2.))).rgb;
  return mix(c*(.25 + 1.1*exp(-trap*2.5)), g, .4);
}
// one tear: which half f came from, or the gap; a glowing torn edge either side
vec2 tear(vec2 f, vec4 T, vec2 n, inout float gap, inout float edge){
  if (abs(T.z) < .5) return f;
  vec2 t = vec2(n.y, n.x);
  float along = dot(f - T.xy, t);
  float jag = (fbm(vec2(along/(90.*u_k), T.w)) - .5)*160.*u_k + (hash12(vec2(floor(along/(2.5*u_k)), T.w)) - .5)*7.*u_k;
  float hf = T.z*.5, d = dot(f - T.xy, n) - jag;
  float dp = d - hf, dm = d + hf;
  if (dp > 0.) { edge += exp(-dp/(2.*u_k)) + .4*exp(-dp/(14.*u_k)); return f - n*hf - t*hf*.22; }
  if (dm < 0.) { edge += exp(dm/(2.*u_k)) + .4*exp(dm/(14.*u_k)); return f + n*hf + t*hf*.22; }
  gap = 1.; return f;
}
void mainImage(out vec4 o, in vec2 f){
  float gap = 0., edge = 0.;
  vec2 src = tear(f, u_tearV, vec2(1., 0.), gap, edge);
  if (gap < .5) src = tear(src, u_tearH, vec2(0., 1.), gap, edge);
  vec4 st = texture(u_str, f/u_size);
  src -= (st.xy*2. - 1.)*st.z*34.*u_k;                  // reality is dragged along the strands
  vec3 fresh = gap > .5 ? hyper(f) : texture(u_scn, src/u_size).rgb;
  fresh += vec3(1., .85, .7)*min(edge, 1.5)*(1. - gap);
  float h = heat(f);
  float col = floor(f.x/(3.*u_k));
  float sp = (.6 + 5.*hash12(vec2(col, 1.)))*u_k*min(h, 2.5);
  vec2 from = f + vec2(sin(f.y*.03/u_k + u_now*2.)*h*1.5*u_k, sp);
  vec3 dripped = texture(u_prev, from/u_size).rgb;
  float keep = smoothstep(.04, .4, h);
  vec3 c = mix(fresh, mix(dripped, fresh, .04), keep);
  float re = smoothstep(.003, .04, h)*(1. - keep);          // cooling: tiles come back in the wrong places first
  if (re > 0.) { vec2 T = floor(f/(48.*u_k)); vec2 jump = (hash22(T + floor(u_now*7.)) - .5)*48.*u_k*4.;
    c = mix(c, texture(u_scn, (src + jump)/u_size).rgb, step(hash12(T + floor(u_now*4.)), re*1.6)); }
  o = vec4(c, 1.);
}`;
const FINAL = `
uniform sampler2D u_m, u_str; uniform float u_mol, u_now, u_k, u_chaos; uniform vec2 u_size;
float smin(float a, float b, float k){ float h = clamp(.5 + .5*(b - a)/k, 0., 1.); return mix(b, a, h) - k*h*(1. - h); }
void mainImage(out vec4 o, in vec2 f){
  vec2 uv = f/u_size, ca = (uv - .5)*u_chaos*.01;
  vec3 base = vec3(texture(u_m, uv + ca).r, texture(u_m, uv).g, texture(u_m, uv - ca).b);
  if (u_mol > .002) {
    float S = 20.*u_k*(1. + .6*vnoise(f/(260.*u_k) + u_now*.05));
    vec2 g = f/S, id = floor(g);
    float dm = 1e9, best = 1e9, br = 1.; vec2 bc = f;
    for (int j = -1; j <= 1; j++) for (int i = -1; i <= 1; i++) {
      vec2 c = id + vec2(float(i), float(j)); float hs = hash12(c);
      float on = smoothstep(hs*.7, hs*.7 + .3, u_mol);
      if (on < .01) continue;
      vec2 ctr = (c + .5 + .3*vec2(sin(u_now*(1. + hs)*2.1 + hs*20.), cos(u_now*(1.3 + hs)*1.7 + hs*9.)))*S;
      float r = S*(.18 + .42*hash12(c + 3.))*on*(.85 + .3*u_mol);
      float d = length(f - ctr) - r;
      dm = smin(dm, d, 7.*u_k);                               // neighbouring globules fuse
      if (d < best) { best = d; bc = ctr; br = max(r, .5); }
    }
    vec2 q = (f - bc)/br; float l = min(length(q), 1.);
    vec3 n = vec3(q/max(1., length(q)), sqrt(1. - l*l));
    vec3 lensC = texture(u_m, (bc - q*br*2.6)/u_size).rgb;     // each globule holds the page, small and upside down
    float spec = pow(max(dot(n, normalize(vec3(-.4, .6, .7))), 0.), 36.), fres = pow(1. - n.z, 2.);
    vec3 irid = .5 + .5*cos(6.283*(fres*.9 + vec3(0., .33, .67) + u_now*.1));
    vec3 bead = lensC*(.6 + .55*n.z) + spec*1.2 + irid*fres*.7;
    float ao = smoothstep(0., 9.*u_k, dm);                      // the page between them, shadowed where they touch it
    vec3 goo = base*mix(.25, .85, ao)*(1. - .35*u_mol) + irid*.06*(1. - ao);
    base = mix(base, mix(bead, goo, smoothstep(-1., 1., dm/u_k)), smoothstep(0., .15, u_mol));
  }
  vec4 st = texture(u_str, uv);
  vec3 sc = .55 + .45*cos(6.283*(st.w + st.z*.5 + vec3(0., .33, .67)));
  vec2 e = 5.*u_k/u_size;
  float halo = .25*(texture(u_str, uv + vec2(e.x, 0.)).z + texture(u_str, uv - vec2(e.x, 0.)).z + texture(u_str, uv + vec2(0., e.y)).z + texture(u_str, uv - vec2(0., e.y)).z);
  base = base*(1. - .3*st.z) + sc*(smoothstep(.12, .85, st.z)*1.25 + st.z*.3 + halo*.45) + smoothstep(.8, 1., st.z)*.5;
  base += (hash12(f + fract(u_now*7.)*311.) - .5)*.05*(.4 + u_chaos);
  o = vec4(base, 1.);
}`;

/* ---------- helpers ---------- */
const esc = s => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const KW = /^(void|float|int|bool|vec2|vec3|vec4|ivec2|mat2|mat3|uniform|const|in|out|inout|if|else|for|return|break|continue|true|false|sampler2D|let|var|new|of|while|function)$/;
function hl(src) {
  let out = '', m; const re = /(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|('(?:\\.|[^'\\])*'|"(?:\\.|[^"\\])*"|`(?:\\.|[^`\\])*`)|(#\w+)|([A-Za-z_]\w*)|(\d+\.?\d*(?:e[-+]?\d+)?|\.\d+)|([\s\S])/g;
  while ((m = re.exec(src))) {
    if (m[1]) out += `<span class="c">${esc(m[1])}</span>`; else if (m[2]) out += `<span class="s">${esc(m[2])}</span>`;
    else if (m[3]) out += `<span class="k">${esc(m[3])}</span>`; else if (m[4]) out += KW.test(m[4]) ? `<span class="k">${m[4]}</span>` : m[4];
    else if (m[5]) out += `<span class="n">${m[5]}</span>`; else out += esc(m[6]);
  }
  return out;
}
function dedent(s) { const lines = s.split('\n'); if (lines.length < 2) return s; const ind = Math.min(...lines.slice(1).filter(l => l.trim()).map(l => l.match(/^ */)[0].length)); return [lines[0], ...lines.slice(1).map(l => l.slice(Math.max(0, ind - 2)))].join('\n'); }

/* ---------- build the page ---------- */
const canvas = document.getElementById('stage'), list = document.getElementById('rifts');
let H = null;
try { H = DS.init(canvas); } catch (e) { list.insertAdjacentHTML('beforebegin', `<p class="nogl">${esc(e.message)} The copy and source are still here.</p>`); }
const E = [];
RIFTS.forEach((g, n) => {
  DS.PALETTES[g.id] = g.pal; g.palette = g.id;
  const glsl = Object.entries(g.glsl).map(([k, v]) => `// ── ${k}.glsl ${'─'.repeat(Math.max(4, 60 - k.length))}\n${v.trim()}`).join('\n\n');
  list.insertAdjacentHTML('beforeend', `<section class="rs${n % 2 ? ' flip' : ''}" id="${g.id}" aria-labelledby="${g.id}-h">
    <div class="in">
      <p class="cap"><span>${String(n + 1).padStart(2, '0')}</span>${esc(g.genre)}</p>
      <h2 id="${g.id}-h" class="ti" style="${g.titleCss || ''}">${g.title.split('\n').map(esc).join('<br>')}</h2>
      <div class="panel">
        <p class="lede">${esc(g.lede)}</p>
        <p class="try"><b>Touch</b> ${esc(g.try)}</p>
        <details class="src"><summary>How it is made</summary>
          <p class="how">${esc(g.how)}</p>
          <div class="tabs" role="tablist"><button type="button" role="tab" aria-selected="true" data-t="glsl">GLSL</button><button type="button" role="tab" aria-selected="false" data-t="js">JS frame()</button></div>
          <pre data-p="glsl">${hl(glsl)}</pre>
          <pre data-p="js" hidden>${hl('frame' + dedent(g.frame.toString()).replace(/^frame/, ''))}</pre>
        </details>
      </div>
      <div class="dev" aria-label="Dual-screen device: ${esc(g.genre)}">
        <div class="scr top" role="img" aria-label="Top screen"></div><div class="hinge" aria-hidden="true"></div>
        <div class="scr bot" role="application" aria-label="Bottom screen: touch it"></div>
      </div>
    </div>
  </section>`);
  const sec = document.getElementById(g.id);
  sec.querySelectorAll('.tabs button').forEach(b => b.addEventListener('click', () => {
    sec.querySelectorAll('.tabs button').forEach(x => x.setAttribute('aria-selected', x === b ? 'true' : 'false'));
    sec.querySelectorAll('.src pre').forEach(p => { p.hidden = p.dataset.p !== b.dataset.t; });
  }));
  if (!H) return;
  const screens = [sec.querySelector('.scr.top'), sec.querySelector('.scr.bot')];
  const run = DS.add(g, { screens, card: sec, offscreen: true }); run.visible = false;
  E.push({ g, run, sec, h2: sec.querySelector('h2'), dev: sec.querySelector('.dev'), panel: sec.querySelector('.panel'), cap: sec.querySelector('.cap'), screens, edge: EDGES[g.edge] ?? 0, ttex: null, tsig: '' });
});

/* ---------- state of the verbs ---------- */
const V = {
  splats: [],                                       // melt heat: [x, y, t, strength] in stage px
  tearV: { amt: 0, vel: 0, key: 0, pad: 0, btn: 0, c: [0, 0], seed: 1 },   // seam runs up and down; halves go left and right
  tearH: { amt: 0, vel: 0, key: 0, pad: 0, btn: 0, c: [0, 0], seed: 2 },
  mol: 0, molKey: 0, molPad: 0, molBtn: 0, meltBtn: 0, meltPad: 0,
  draw: { down: false, p0: [0, 0], p1: [0, 0], fresh: false }, clear: 2, now: 0
};
const ROWS = ['1234567890-=', 'qwertyuiop[]', "asdfghjkl;'\\", 'zxcvbnm,./'];
const keyPos = k => { k = k.toLowerCase(); for (let r = 0; r < 4; r++) { const c = ROWS[r].indexOf(k); if (c >= 0) return [(c + r * 0.4 + 0.5) / 12.8, (r + 0.5) / 4]; } return [Math.random(), Math.random()]; };
let K = 1, SW = 0, SH = 0;
function splat(u, v, s) { V.splats.push([u * SW, (1 - v) * SH, V.now, s]); while (V.splats.length > 16) V.splats.shift(); }
const typing = e => /INPUT|SELECT|TEXTAREA/.test(e.target.tagName);
addEventListener('keydown', e => {
  if (typing(e) || e.metaKey || e.ctrlKey || e.altKey) return;
  if (e.key === ' ') { V.molKey = 1; e.preventDefault(); return; }
  if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') { V.tearV.key = 1; e.preventDefault(); return; }
  if (e.key === 'ArrowUp' || e.key === 'ArrowDown') { V.tearH.key = 1; e.preventDefault(); return; }
  if (e.key.length === 1 || e.key === 'Enter' || e.key === 'Backspace') { const [u, v] = keyPos(e.key); splat(u, v, e.repeat ? 0.45 : 1); if (e.key === "'" || e.key === '/') e.preventDefault(); }
});
addEventListener('keyup', e => {
  if (e.key === ' ') V.molKey = 0;
  if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') V.tearV.key = 0;
  if (e.key === 'ArrowUp' || e.key === 'ArrowDown') V.tearH.key = 0;
});
addEventListener('blur', () => { V.molKey = 0; V.tearV.key = V.tearH.key = 0; });
/* drawing: drag anywhere that is not text or a control */
const mouse = [0, 0, 0];
const drawable = t => !t.closest('.panel, button, a, input, select, summary, .verbs, .bar');
addEventListener('pointermove', e => {
  mouse[0] = e.clientX; mouse[1] = e.clientY;
  if (V.draw.down) { V.draw.p1 = [e.clientX * K, (innerHeight - e.clientY) * K]; }
}, { passive: true });
addEventListener('pointerdown', e => {
  mouse[2] = 1; mouse[0] = e.clientX; mouse[1] = e.clientY;
  if (!drawable(e.target)) return;
  V.draw.down = true; V.draw.p0 = V.draw.p1 = [e.clientX * K, (innerHeight - e.clientY) * K];
  if (e.target === document.documentElement || e.target === document.body || e.target.closest('main')) e.preventDefault();
});
const up = () => { mouse[2] = 0; V.draw.down = false; };
addEventListener('pointerup', up); addEventListener('pointercancel', up);
/* the verb buttons, for touch screens and anyone without a keyboard */
document.querySelectorAll('.verbs [data-v]').forEach(b => {
  const on = v => { const k = b.dataset.v; if (k === 'melt') V.meltBtn = v; if (k === 'tearV') V.tearV.btn = v; if (k === 'tearH') V.tearH.btn = v; if (k === 'mol') V.molBtn = v; b.setAttribute('aria-pressed', v ? 'true' : 'false'); };
  b.addEventListener('pointerdown', e => { b.setPointerCapture(e.pointerId); on(1); e.preventDefault(); });
  for (const ev of ['pointerup', 'pointercancel', 'lostpointercapture']) b.addEventListener(ev, () => on(0));
  b.addEventListener('keydown', e => { if (e.key === 'Enter') { on(1); setTimeout(() => on(0), 700); } });
});
const mend = document.getElementById('mend');
if (mend) mend.addEventListener('click', () => { V.splats = []; V.mol = 0; V.clear = 2; for (const T of [V.tearV, V.tearH]) { T.amt = T.vel = 0; } });
/* a real gamepad, if the browser lets the page see one */
function pollPad() {
  V.tearV.pad = V.tearH.pad = V.molPad = V.meltPad = 0;
  let pads = []; try { pads = navigator.getGamepads ? [...navigator.getGamepads()].filter(Boolean) : []; } catch (e) { return; }
  const p = pads[0]; if (!p) return;
  const ax = p.axes || [], bt = i => (p.buttons[i] && (p.buttons[i].pressed ? Math.max(0.6, p.buttons[i].value) : 0)) || 0;
  V.tearV.pad = Math.max(Math.abs(ax[0] || 0) > 0.15 ? Math.abs(ax[0]) : 0, bt(14), bt(15));
  V.tearH.pad = Math.max(Math.abs(ax[1] || 0) > 0.15 ? Math.abs(ax[1]) : 0, bt(12), bt(13));
  V.molPad = Math.max(bt(0), bt(1), bt(2), bt(3));
  V.meltPad = Math.max(bt(6), bt(7), bt(4), bt(5));
}

/* ---------- the stage ---------- */
let pool = [], fontsV = 0, running = !matchMedia('(prefers-reduced-motion: reduce)').matches, scn = null, mA = null, mB = null, sA = null, sB = null, lastMelt = 0;
const QUALITY = { sharp: () => Math.min(2, devicePixelRatio || 1), normal: () => 1, fast: () => 0.6 };
let quality = 'normal';
if (document.fonts) { document.fonts.ready.then(() => { fontsV++; for (const e of E) { for (const [k, v] of e.run.tex) if (v.sig) e.run.tex.delete(k); e.run.dirty = true; } }); }
function free(T) { if (T) { H.gl.deleteTexture(T.tex); H.gl.deleteFramebuffer(T.fb); } }
function resize() {
  K = QUALITY[quality]();
  const w = Math.max(2, Math.round(innerWidth * K)), h = Math.max(2, Math.round(innerHeight * K));
  if (w === SW && h === SH) return;
  for (const T of [...pool, scn, mA, mB, sA, sB]) free(T);
  pool = []; SW = canvas.width = w; SH = canvas.height = h;
  scn = H.target(w, h, { linear: true }); mA = H.target(w, h, { linear: true }); mB = H.target(w, h, { linear: true });
  sA = H.target(w >> 1, h >> 1, { linear: true }); sB = H.target(w >> 1, h >> 1, { linear: true }); V.clear = 2;
  V.tearV.c = [w / 2, h / 2]; V.tearH.c = [w / 2, h / 2];
}
const slot = j => pool[j] || (pool[j] = H.target(SW, SH));
const rectUp = b => [b.left * K, (innerHeight - b.bottom) * K, b.width * K, b.height * K];
const progs = {};
const prog = (key, src) => progs[key] || (progs[key] = H.program(src, []));
function pass(P, T, w, h, uniforms, textures) {
  const gl = H.gl; gl.bindFramebuffer(gl.FRAMEBUFFER, T ? T.fb : null); gl.viewport(0, 0, w, h); gl.disable(gl.BLEND);
  gl.useProgram(P.p); gl.bindVertexArray(H.vao);
  H.set(P, 'iResolution', [w, h, 1]); H.set(P, 'iTime', V.now % 600);
  for (const [k, v] of Object.entries(uniforms)) H.set(P, k, v);
  let u = 0; for (const [k, t] of Object.entries(textures)) { gl.activeTexture(gl.TEXTURE0 + u); gl.bindTexture(gl.TEXTURE_2D, t.tex || t); H.set(P, k, u); u++; }
  gl.drawArrays(gl.TRIANGLES, 0, 3);
}

function titleTex(e) {
  const r = e.h2.getBoundingClientRect(), cs = getComputedStyle(e.h2), fs = parseFloat(cs.fontSize), pad = Math.round(fs * 0.5);
  const b = { left: r.left, top: r.top, bottom: r.bottom, width: Math.max(r.width, e.h2.scrollWidth), height: r.height };   // the text may overhang its grid box
  e.rTitle = rectUp({ left: b.left - pad, bottom: b.bottom + pad, width: b.width + 2 * pad, height: b.height + 2 * pad });
  const sig = [e.g.title, cs.fontFamily, cs.fontWeight, cs.fontStyle, fs, cs.lineHeight, cs.letterSpacing, Math.round(b.width), Math.round(b.height), K, fontsV].join('|');
  if (sig === e.tsig) return e.ttex;
  const c = document.createElement('canvas'), w = Math.ceil((b.width + 2 * pad) * K), h = Math.ceil((b.height + 2 * pad) * K);
  c.width = w; c.height = h;
  const x = c.getContext('2d'), OFF = 30000, px = fs * K, lines = e.g.title.split('\n'), lh = (cs.lineHeight === 'normal' ? fs * 1.15 : parseFloat(cs.lineHeight)) * K;
  x.fillStyle = '#000'; x.fillRect(0, 0, w, h); x.font = `${cs.fontStyle} ${cs.fontWeight} ${px}px ${cs.fontFamily}`; x.textBaseline = 'middle'; x.textAlign = 'left';
  if ('letterSpacing' in x) x.letterSpacing = (parseFloat(cs.letterSpacing) || 0) * K + 'px';
  const at = i => [pad * K, pad * K + (i + 0.5) * lh];
  x.globalCompositeOperation = 'lighter';
  const shadow = (col, blur) => { x.save(); x.shadowColor = col; x.shadowBlur = blur; x.shadowOffsetX = OFF; x.fillStyle = col; lines.forEach((l, i) => { const [a, y] = at(i); x.fillText(l, a - OFF, y); }); x.restore(); };
  shadow('#00ff00', Math.max(3, px * 0.08)); shadow('#0000ff', px * 0.5);
  x.fillStyle = '#ff0000'; lines.forEach((l, i) => { const [a, y] = at(i); x.fillText(l, a, y); });
  e.ttex = H.texFromCanvas(c, e.ttex); e.tsig = sig; return e.ttex;
}

let chaos = 0;
function drawSection(e, j, b) {
  const gl = H.gl, r = e.run, T = slot(j);
  gl.bindFramebuffer(gl.FRAMEBUFFER, T.fb); gl.viewport(0, 0, SW, SH);
  const y0 = Math.max(0, innerHeight - b.bottom - MARGIN), y1 = Math.min(innerHeight, innerHeight - b.top + MARGIN);
  gl.enable(gl.SCISSOR_TEST); gl.scissor(0, Math.floor(y0 * K), SW, Math.ceil((y1 - y0) * K) + 2); gl.disable(gl.BLEND);
  const tt = titleTex(e), rT = rectUp(e.screens[0].getBoundingClientRect()), rB = rectUp(e.screens[1].getBoundingClientRect());
  const P = e.run.H.program(STAGE_HEAD + '\n#line 1\n' + e.g.glsl.bg, e.g.params || []);
  r.bind(P, { uniforms: Object.assign({ iStage: [SW, SH], u_k: K, u_band: rectUp(b), iScroll: Math.max(0, Math.min(1, (innerHeight - b.top) / (innerHeight + b.height))),
    iMouse: [mouse[0] * K, (innerHeight - mouse[1]) * K, mouse[2], 0], u_rTop: rT, u_rBot: rB, u_rTitle: e.rTitle, u_chaos: chaos }, r.bgUniforms || {}),
    textures: Object.assign({ u_scrTop: r.scr[0] || H.white, u_scrBot: r.scr[1] || H.white, u_title: tt }, r.bgTextures || {}) }, 0, [SW, SH, 1]);
  gl.drawArrays(gl.TRIANGLES, 0, 3);
  const d = e.dev.getBoundingClientRect(), m = 60;
  const sx = Math.max(0, (d.left - m) * K), sy = Math.max(0, (innerHeight - d.bottom - m) * K);
  const sw = Math.min(SW, (d.right + m) * K) - sx, sh = Math.min(SH, (innerHeight - d.top + m) * K) - sy;
  if (sw > 0 && sh > 0) {
    gl.scissor(Math.floor(sx), Math.floor(sy), Math.ceil(sw), Math.ceil(sh)); gl.enable(gl.BLEND); gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
    r.bind(prog('device', DEVICE), { uniforms: { u_rTop: rT, u_rBot: rB, u_k: K, u_shell: e.g.shell || [0.08, 0.08, 0.1] }, textures: { u_scrTop: r.scr[0] || H.white, u_scrBot: r.scr[1] || H.white } }, 0, [SW, SH, 1]);
    gl.drawArrays(gl.TRIANGLES, 0, 3); gl.disable(gl.BLEND);
  }
  gl.disable(gl.SCISSOR_TEST);
}
function composite(vis) {
  const gl = H.gl, S = prog('seam', SEAM);
  gl.bindFramebuffer(gl.FRAMEBUFFER, scn.fb); gl.viewport(0, 0, SW, SH);
  if (!vis.length) { gl.clearColor(0, 0, 0, 1); gl.clear(gl.COLOR_BUFFER_BIT); }
  vis.forEach(([e, b], j) => {
    gl.useProgram(S.p); gl.bindVertexArray(H.vao);
    H.set(S, 'iResolution', [SW, SH, 1]); H.set(S, 'iTime', V.now % 600);
    H.set(S, 'u_edge', (innerHeight - b.top) * K); H.set(S, 'u_first', j === 0 ? 1 : 0); H.set(S, 'u_k', K);
    H.set(S, 'u_kind', e.edge); H.set(S, 'u_glow', e.run.rgb.hot);
    gl.activeTexture(gl.TEXTURE0); gl.bindTexture(gl.TEXTURE_2D, pool[j].tex); H.set(S, 'u_sec', 0);
    if (j) { gl.enable(gl.BLEND); gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA); } else gl.disable(gl.BLEND);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  });
  gl.disable(gl.BLEND);
}

/* the verbs over time: springs, holds, the strands' pen */
function verbs(dt) {
  pollPad();
  for (const T of [V.tearV, V.tearH]) {
    const want = Math.max(T.key, T.pad, T.btn);
    if (want > 0 && Math.abs(T.amt) < 4 && Math.abs(T.vel) < 40) { T.c = [mouse[0] * K || SW / 2, (innerHeight - mouse[1]) * K || SH / 2]; if (!mouse[0]) T.c = [SW / 2, SH / 2]; T.seed = Math.random() * 50; }
    const target = want * 0.44 * SH;
    T.vel += ((target - T.amt) * 70 - T.vel * 6.5) * dt; T.amt += T.vel * dt;
    if (!want && Math.abs(T.amt) < 0.5 && Math.abs(T.vel) < 5) T.amt = T.vel = 0;
  }
  const molWant = Math.max(V.molKey, V.molPad, V.molBtn);
  V.mol += (molWant - V.mol) * Math.min(1, dt * (molWant > V.mol ? 1.6 : 0.45));
  if (V.mol < 0.002 && !molWant) V.mol = 0;
  const meltWant = Math.max(V.meltBtn, V.meltPad);
  if (meltWant && V.now - lastMelt > 0.09) { lastMelt = V.now; splat(Math.random(), Math.random() * 0.8, 0.8 * meltWant); }
  let heatSum = 0; for (const s of V.splats) heatSum += s[3] * Math.exp(-(V.now - s[2]) * 0.3);
  V.splats = V.splats.filter(s => s[3] * Math.exp(-(V.now - s[2]) * 0.3) > 0.004);
  chaos = Math.min(3, heatSum * 0.5 + (Math.abs(V.tearV.amt) + Math.abs(V.tearH.amt)) / SH * 2 + V.mol * 1.5);
  document.documentElement.style.setProperty('--chaos', chaos.toFixed(3));
  const meters = { melt: Math.min(1, heatSum / 3), tear: Math.min(1, (Math.abs(V.tearV.amt) + Math.abs(V.tearH.amt)) / (0.44 * SH || 1)), mol: V.mol };
  for (const [k, v] of Object.entries(meters)) { const el = document.querySelector(`.verbs [data-m="${k}"]`); if (el) el.style.setProperty('--m', v.toFixed(3)); }
  return heatSum;
}
/* the DOM rides along: a panel goes with its half of each tear, sags where it is hot, blurs into molecules */
function heatAt(x, y) { let h = 0; for (const s of V.splats) { const dx = (x - s[0]) / (SH * 0.2), dy = (y - s[1]) / (SH * 0.2); h += s[3] * Math.exp(-(dx * dx + dy * dy)) * Math.exp(-(V.now - s[2]) * 0.3); } return h; }
function ride(vis) {
  for (const [e] of vis) for (const el of [e.panel, e.cap]) {
    const b = el.getBoundingClientRect(); if (!b.width) continue;
    const cx = (b.left + b.width / 2) * K, cy = (innerHeight - b.top - b.height / 2) * K;
    let dx = 0, dy = 0;
    if (Math.abs(V.tearV.amt) > 0.5) { const s = cx > V.tearV.c[0] ? 1 : -1; dx += s * V.tearV.amt / 2; dy -= s * V.tearV.amt * 0.11; }
    if (Math.abs(V.tearH.amt) > 0.5) { const s = cy > V.tearH.c[1] ? 1 : -1; dy -= s * V.tearH.amt / 2; dx += s * V.tearH.amt * 0.11; }
    const h = Math.min(2.5, heatAt(cx, cy)), m = V.mol;
    const tf = (dx || dy || h > 0.01) ? `translate(${(dx / K).toFixed(1)}px, ${(dy / K + h * 46).toFixed(1)}px) skewY(${(h * 5).toFixed(2)}deg)` : '';
    const fl = (h > 0.01 || m > 0.01) ? `blur(${(h * 1.6 + m * 3).toFixed(2)}px) saturate(${(1 + h + m).toFixed(2)})` : '';
    if (el.style.transform !== tf) el.style.transform = tf;
    if (el.style.filter !== fl) el.style.filter = fl;
  }
}

if (H) {
  DS.onError = (r, err) => { const e = E.find(x => x.run === r); if (!e) return; let p = e.sec.querySelector('.err'); if (!p) { p = document.createElement('pre'); p.className = 'err'; e.panel.prepend(p); } p.textContent = 'This section stopped: ' + err.message; };
  DS.after = (dt) => {
    DS.focus = null;                                 // the keyboard belongs to the verbs here, not to the consoles
    resize();
    if (running) V.now += dt;
    verbs(running ? dt : 0);
    const vis = [];
    for (const e of E) { const b = e.sec.getBoundingClientRect(), on = b.bottom > -MARGIN && b.top < innerHeight + MARGIN; e.run.visible = on; if (on) vis.push([e, b]); }
    vis.forEach(([e, b], j) => { try { drawSection(e, j, b); } catch (err) { if (!e.run.error) { e.run.error = err; DS.onError(e.run, err); console.error(e.g.id, err); } } });
    composite(vis);
    const d = V.draw, hue = (V.now * 0.11) % 1;
    pass(prog('strands', STRANDS), sB, sB.w, sB.h, { u_size: [sB.w, sB.h], u_k2: K * 0.5, u_down: d.down ? 1 : 0, u_hue: hue, u_clear: V.clear > 0 ? 1 : 0,
      u_seg: [d.p0[0] * 0.5, d.p0[1] * 0.5, d.p1[0] * 0.5, d.p1[1] * 0.5] }, { u_prev: sA });
    [sA, sB] = [sB, sA]; d.p0 = d.p1;
    const spl = []; for (let i = 0; i < 16; i++) { const s = V.splats[i]; spl.push(...(s || [0, 0, 0, 0])); }
    pass(prog('melt', MELT), mB, SW, SH, { u_size: [SW, SH], u_k: K, u_now: V.now, u_spl: spl,
      u_tearV: [V.tearV.c[0], V.tearV.c[1], V.tearV.amt, V.tearV.seed], u_tearH: [V.tearH.c[0], V.tearH.c[1], V.tearH.amt, V.tearH.seed] }, { u_scn: scn, u_prev: V.clear > 0 ? scn : mA, u_str: sA });
    [mA, mB] = [mB, mA];
    pass(prog('final', FINAL), null, SW, SH, { u_size: [SW, SH], u_k: K, u_now: V.now, u_mol: V.mol, u_chaos: chaos }, { u_m: mA, u_str: sA });
    if (V.clear > 0) V.clear--;
    ride(vis);
  };
  const setRun = on => { running = on; for (const e of E) { e.run.running = on; e.run.dirty = true; } const b = document.getElementById('motion'); if (b) { b.textContent = on ? 'Freeze' : 'Thaw'; b.setAttribute('aria-pressed', on ? 'false' : 'true'); } };
  setRun(running);
  const mb = document.getElementById('motion'); if (mb) mb.addEventListener('click', () => setRun(!running));
  const q = document.getElementById('quality'); if (q) q.addEventListener('change', () => { quality = q.value; });
  DS.start();
}
window.RIFT = { E, V, STAGE_HEAD };
})();
