/* phant.js — Lattice Phantasmagoria: a console whose own furniture comes apart. One dshost sketch drives both screens.

   The idea mirrors no_shade. no_shade kept a real web page (text, links, scroll) and let shaders turn each element
   into a material. Here the console keeps a real UI: A confirms, B backs out, HP drains, the save files and inventory are
   state. Each screen's UI is drawn plainly into a 2D layer, the way Lattice draws its widget tree, and the shaders only
   get that picture (red ink, green panels, blue focus) plus the state.

   Per frame:
     1. input → the verbs (keys melt, L/R tear the hinge, X molecules, Y Ubik, stylus strands) and reality's half-life;
     2. the scene's update (and the outgoing scene's, during a transition);
     3. UI layers redrawn when their signature changes → mipmapped textures;
     4. each screen's scene shader at the internal resolution (256×192 like the DS, or 512×384);
     5. STRANDS: one memory over both screens and the hinge (512×860 logical px at half resolution);
     6. COMP per screen, with its own last frame as feedback: tear, strands drag, reality regression, transition,
        status bar, melt, molecules, strand glow, Ubik;
     7. present: nearest-neighbour ×4 (or ×2) onto the panel. */
(function () {
'use strict';
const DS = window.DS, PH = window.PHANT;
const LW = DS.LW, LH = DS.LH, GAP = DS.GAP, CHT = 2 * LH + GAP;
const FONTS = { sil: '"Silkscreen", "Courier New", monospace', vt: '"VT323", "Courier New", monospace', jac: '"Jacquard 24", Georgia, serif', cou: '"Courier Prime", "Courier New", monospace' };
Object.assign(DS.KEYMAP, { X: 'A', Z: 'B', S: 'X', A: 'Y', Q: 'L', W: 'R', Shift: 'SELECT', Escape: 'START', ' ': 'A' });

/* ---------- a UI layer: a 2D canvas in logical px (y down), red ink, green panels, blue focus ---------- */
class Layer {
  constructor(w = LW, h = LH, k = 2) { this.c = document.createElement('canvas'); this.c.width = w * k; this.c.height = h * k; this.k = k; this.w = w; this.h = h; this.x = this.c.getContext('2d'); this.tex = null; this.sig = null; }
  begin() { const x = this.x; x.setTransform(1, 0, 0, 1, 0, 0); x.globalCompositeOperation = 'source-over'; x.fillStyle = '#000'; x.fillRect(0, 0, this.c.width, this.c.height);
    x.setTransform(this.k, 0, 0, this.k, 0, 0); x.globalCompositeOperation = 'lighter'; return this; }
  col(ch, a = 1) { const v = Math.round(255 * Math.max(0, Math.min(1, a))); return `rgb(${ch.includes('r') ? v : 0},${ch.includes('g') ? v : 0},${ch.includes('b') ? v : 0})`; }
  font(f, size, w) { this.x.font = `${w || 400} ${size}px ${FONTS[f] || f}`; }
  text(s, x, y, o = {}) {
    const X = this.x; this.font(o.f || 'sil', o.size || 16, o.w); X.textAlign = o.align || 'left'; X.textBaseline = o.base || 'alphabetic';
    if ('letterSpacing' in X) X.letterSpacing = (o.track || 0) + 'px';
    X.fillStyle = this.col(o.ch || 'r', o.a ?? 1);
    if (o.rot) { X.save(); X.translate(x, y); X.rotate(o.rot); X.fillText(s, 0, 0); X.restore(); } else X.fillText(s, x, y);
    const w = X.measureText(s).width; if ('letterSpacing' in X) X.letterSpacing = '0px'; return w;
  }
  measure(s, o = {}) { this.font(o.f || 'sil', o.size || 16, o.w); return this.x.measureText(s).width; }
  lines(s, w, o = {}) { this.font(o.f || 'vt', o.size || 20, o.w); const out = []; for (const para of String(s).split('\n')) { let line = '';
      for (const wd of para.split(' ')) { const t = line ? line + ' ' + wd : wd; if (this.x.measureText(t).width > w && line) { out.push(line); line = wd; } else line = t; }
      out.push(line); } return out; }
  wrap(s, x, y, w, o = {}) { const lh = o.lh || (o.size || 20) * 1.1, L = this.lines(s, w, o); L.forEach((l, i) => this.text(l, x, y + i * lh, o)); return y + L.length * lh; }
  rect(x, y, w, h, o = {}) { const X = this.x; X.fillStyle = this.col(o.ch || 'g', o.a ?? 1); X.beginPath(); if (o.r) X.roundRect(x, y, w, h, o.r); else X.rect(x, y, w, h); X.fill(); }
  frame(x, y, w, h, o = {}) { const X = this.x; X.strokeStyle = this.col(o.ch || 'r', o.a ?? 1); X.lineWidth = o.lw || 2; X.beginPath(); if (o.r) X.roundRect(x, y, w, h, o.r); else X.rect(x, y, w, h); X.stroke(); }
  path(fn, o = {}) { const X = this.x; X.beginPath(); fn(X); if (o.fill) { X.fillStyle = this.col(o.ch || 'r', o.a ?? 1); X.fill(); } else { X.strokeStyle = this.col(o.ch || 'r', o.a ?? 1); X.lineWidth = o.lw || 2; X.stroke(); } }
  upload(H) {
    const gl = H.gl; this.tex = H.texFromCanvas(this.c, this.tex);
    gl.bindTexture(gl.TEXTURE_2D, this.tex); gl.generateMipmap(gl.TEXTURE_2D); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
    return this.tex;
  }
}
PH.Layer = Layer;

/* ---------- GLSL shared by every scene and by COMP ---------- */
const LIB = `
const vec2 LS = vec2(${LW}., ${LH}.);        // one screen, logical px
const vec2 CS = vec2(${LW}., ${CHT}.);       // both screens and the hinge
uniform float u_life;                        // reality left, 1 → 0 (Ubik's half-life)
uniform float u_now;                         // seconds, not wrapped
mat2 rot2(float a){ float c = cos(a), s = sin(a); return mat2(c, s, -s, c); }
float sdBox(vec2 p, vec2 b){ vec2 d = abs(p) - b; return length(max(d, 0.)) + min(max(d.x, d.y), 0.); }
float sdSeg(vec2 p, vec2 a, vec2 b){ vec2 pa = p - a, ba = b - a; return length(pa - ba*clamp(dot(pa, ba)/max(dot(ba, ba), 1e-4), 0., 1.)); }
float smin(float a, float b, float k){ float h = clamp(.5 + .5*(b - a)/k, 0., 1.); return mix(b, a, h) - k*h*(1. - h); }
vec2 curl(vec2 p){ float e = .12; return vec2(vnoise(p + vec2(0., e)) - vnoise(p - vec2(0., e)), vnoise(p - vec2(e, 0.)) - vnoise(p + vec2(e, 0.)))/(2.*e); }
vec3 spec(float h){ return .5 + .5*cos(6.2832*(h + vec3(0., .33, .67))); }
float luma(vec3 c){ return dot(c, vec3(.299, .587, .114)); }
float bayer2(vec2 a){ a = floor(a); return fract(dot(a, vec2(.5, a.y*.75))); }
float bayer4(vec2 a){ return bayer2(.5*a)*.25 + bayer2(a); }
// reality running out, era by era: 0 true colour · 1 sixty-four colours · 2 the four tokens · 3 one bit · 4 punch card · 5 nothing
float eraAt(vec2 c, float life){ return clamp((1. - life)*5.8 - .4 + (fbm(c*.009 + 7.3) - .5)*1.7, 0., 5.); }
const vec2 CARD = vec2(${(LW / 40).toFixed(2)}, ${(LH / 12).toFixed(2)});   // 40 columns × 12 rows per screen
vec2 eraSrc(vec2 p, float e){ int k = int(e);
  if (k == 2) return (floor(p/4.) + .5)*4.;
  if (k == 3) return (floor(p/8.) + .5)*8.;
  if (k == 4) return (floor(p/CARD) + .5)*CARD;
  return p; }
vec3 eraCol(vec3 c, vec2 p, float e, vec2 px){ int k = int(e); float b = bayer4(px) - .5;
  if (k == 0) return c;
  if (k == 1) return clamp(floor(c*3. + .5 + b*.9)/3., 0., 1.);
  if (k == 2) { float L = luma(c) + b*.28; return L < .2 ? u_ground : L < .42 ? u_cool : L < .66 ? u_hot : u_ink; }
  if (k == 3) return luma(c) + b*.7 > .4 ? u_ink : u_ground;
  if (k == 4) { vec2 q = fract(p/CARD); vec3 card = mix(u_ink, vec3(.93, .84, .62), .55);
    float hole = step(abs(q.x - .5), .2)*step(abs(q.y - .5), .3)*step(.4, luma(c));
    float printed = step(abs(q.x - .5), .14)*step(abs(q.y - .5), .2)*step(.5, hash12(floor(p/CARD) + 3.))*.08;
    return mix(card - printed, u_ground*.5, hole); }
  float n = hash12(floor(px) + fract(u_now*17.)*vec2(91., 37.)); return mix(u_ground*.6, u_ink*.55, n*n*n); }
`;
const SCENE_HEAD = `
uniform float u_t, u_in, u_sel;              // scene time, entrance 0→1, selection
uniform vec4 u_p0, u_p1, u_p2, u_p3;         // scene numbers
uniform sampler2D u_ui, u_mem, u_aux, u_aux2;
${LIB}
float UILOD(){ return log2(2./iResolution.z); }
vec3 ui(vec2 f){ return textureLod(u_ui, f/LS, UILOD()).rgb; }        // this screen's UI layer: r ink, g panels, b focus
vec3 uiL(vec2 f, float l){ return textureLod(u_ui, f/LS, UILOD() + l).rgb; }
vec2 ipx(vec2 f){ return floor(f*iResolution.z); }                    // the internal pixel under f
float aa(float d){ return clamp(.5 - d*iResolution.z, 0., 1.); }        // coverage of a distance in logical px
`;

const STRANDS = `
${LIB}
uniform vec4 u_seg; uniform float u_down, u_hue, u_clear, u_fade;
void mainImage(out vec4 o, in vec2 f){
  vec2 size = iResolution.xy;
  if (u_clear > .5) { o = vec4(.5, .5, 0., 0.); return; }
  vec2 v0 = texture(iChannel0, f/size).xy*2. - 1.;
  vec2 c = curl(f/size.y*3.5 + iTime*.06);
  vec2 back = f - (v0*1.1 + c*.9 + vec2(0., .22));        // strands creep along themselves, bend with the noise, rise toward the top screen
  vec4 s = texture(iChannel0, back/size);
  vec2 v = normalize(mix(s.xy*2. - 1., normalize(c + 1e-4), .025) + 1e-4);
  float dens = s.z*.997 - .0004 - u_fade*.04, hue = s.w;
  float side = texture(iChannel0, (back + vec2(-v.y, v.x)*2.*sin(iTime*3. + f.x*.05))/size).z;
  dens = max(dens, side*.985*step(.9, vnoise(f*.08 + iTime)));
  if (u_down > .5) {
    float d = sdSeg(f, u_seg.xy, u_seg.zw), m = exp(-d*d/4.5);
    vec2 dir = normalize(u_seg.zw - u_seg.xy + 1e-4);
    dens = max(dens, m); v = normalize(mix(v, dir, m) + 1e-4); hue = mix(hue, u_hue, m);
  }
  o = vec4(v*.5 + .5, clamp(dens, 0., 1.), hue);
}`;

const COMP = `
${LIB}
uniform sampler2D u_a, u_b, u_str, u_status;
uniform float u_tr, u_kind, u_tear, u_mol, u_ubik, u_chaos, u_sb, u_seed;
uniform vec4 u_focus;            // transition focus: top screen xy, bottom screen zw (logical px)
uniform vec4 u_heat[12];         // melt: canvas x, y, start time, strength
vec3 scn(sampler2D t, vec2 L){ return texture(t, clamp(L/LS, .0005, .9995)).rgb; }
float heat(vec2 C){ float h = 0.;
  for (int i = 0; i < 12; i++) { vec4 s = u_heat[i]; if (s.w <= 0.) continue; vec2 d = (C - s.xy)/78.; h += s.w*exp(-dot(d, d))*exp(-(u_now - s.z)*.5); }
  return h; }
// what is under the console: its own picture folded through a kaleidoscope and a foam of circle inversions
vec3 hyper(vec2 C){
  vec2 p = (C - .5*CS)/CS.x*2.6; float t = u_now*.13;
  float a = atan(p.y, p.x), r = length(p), k = 2.*PI/7.; a = abs(mod(a + t, k) - k*.5); p = r*vec2(cos(a), sin(a));
  float trap = 1e9;
  for (int i = 0; i < 7; i++) { p = abs(p) - vec2(.55, .32); float m = clamp(.75/dot(p, p), 1., 3.5); p *= m; trap = min(trap, abs(length(p) - .7)); p = rot2(t)*p; }
  vec3 c = mixOk(u_hot, u_cool, .5 + .5*sin(trap*5. + u_now*.4))*(.2 + 1.2*exp(-trap*2.5)) + spec(trap*1.4 + u_now*.07)*.25*exp(-trap*4.);
  vec3 g = texture(u_a, 1. - abs(1. - mod(p*.05 + .5, 2.))).rgb;
  return mix(c, g, .3);
}
// the hinge tears: each screen's picture is pushed away from it, and the gap shows hyperreality
vec2 tearSrc(vec2 L, inout float gap, inout float edge){
  if (u_tear < .003) return L;
  float w = u_tear*150. + (fbm(vec2(L.x*.013, 4.2 + u_seed)) - .5)*110.*u_tear + (hash12(vec2(floor(L.x/3.), 9.)) - .5)*7.*u_tear;
  w = max(w, 0.);
  float d = (iScreen == 0 ? L.y : LS.y - L.y) - w;
  if (d < 0.) { gap = 1.; edge += exp(d/2.); return L; }
  edge += exp(-d/2.) + .4*exp(-d/14.);
  return iScreen == 0 ? L - vec2(w*.12, w) : L + vec2(w*.12, w);
}
void mainImage(out vec4 o, in vec2 f){
  vec2 L = f*LS/iResolution.xy, C = dsCanvas(L);
  float gap = 0., edge = 0.;
  vec2 src = tearSrc(L, gap, edge);
  vec4 st = texture(u_str, C/CS);
  src -= (st.xy*2. - 1.)*st.z*14.;                                   // reality is dragged along the strands
  float k = u_tr; int kind = int(u_kind + .5);
  float eT = (kind == 1 && k < 1.) ? 5.4*(1. - abs(2.*k - 1.)) : 0.;  // "regress": the old scene runs out, the new one comes back
  float e = max(eraAt(C, u_life), eT);
  vec2 s = eraSrc(src, e);
  bool useB = false; vec2 sA = s, sB = s; float band = 1e9;
  if (k < 1.) {
    if (kind == 1) useB = k < .5;
    else if (kind == 2) { float y = mix(CS.y + 60., -60., k); band = C.y - y; useB = band < 0.; }   // a paper tape feeds through both screens
    else if (kind == 3) {                                              // the new scene opens out of the thing you chose
      vec2 F = iScreen == 0 ? u_focus.xy : u_focus.zw; float z = mix(.04, 1., k*k*(3. - 2.*k));
      sA = F + (s - F)/z; useB = any(lessThan(sA, vec2(0.))) || any(greaterThan(sA, LS)); sB = F + (s - F)/(1. + 4.*k); }
    else if (kind == 4) useB = hash12(floor(C/24.) + u_seed) > k*1.1 - .05;   // scramble: cell by cell
  }
  vec3 fresh;
  if (gap > .5) fresh = hyper(C);
  else {
    vec2 q = useB ? sB : sA; float ch = min(u_chaos, 3.)*1.3;
    fresh = useB ? scn(u_b, q) : scn(u_a, q);
    if (ch > .3) { fresh.r = (useB ? scn(u_b, q + vec2(ch, 0.)) : scn(u_a, q + vec2(ch, 0.))).r; fresh.b = (useB ? scn(u_b, q - vec2(ch, 0.)) : scn(u_a, q - vec2(ch, 0.))).b; }
    if (iScreen == 0 && u_sb > .01 && s.y > LS.y - 30.) {             // the status bar: system UI, and just as breakable
      vec3 sb = texture(u_status, vec2(s.x/LS.x, (s.y - (LS.y - 30.))/30.)).rgb;
      fresh = mix(fresh, u_ground*.72, sb.g*.88*u_sb); fresh = mix(fresh, u_hot, sb.b*u_sb); fresh = mix(fresh, u_ink, sb.r*u_sb);
    }
    fresh = eraCol(fresh, src, e, f);
  }
  if (abs(band) < 22.) {                                               // the tape itself, with its feed holes
    float hx = mod(C.x, 11.) - 5.5, hy = band;
    fresh = mix(vec3(.93, .86, .66)*(.85 + .15*hash12(vec2(floor(C.x/11.), 2.))), u_ground*.4, step(length(vec2(hx, hy - 9.)), 2.2) + step(length(vec2(hx, hy + 6.)), 3.4)*step(.5, hash12(vec2(floor(C.x/11.), floor(u_now*6.)))));
  }
  fresh += mixOk(u_hot, vec3(1., .9, .75), .5)*min(edge, 1.5)*(1. - gap);
  vec3 c = fresh;
  float h = heat(C);                                                   // melt: hot pixels keep sliding down their own last frame
  if (h > .003) {
    float rs = iResolution.x/LS.x;
    float col = floor(f.x/max(1., 3.*rs));
    float sp = (.3 + 2.6*hash12(vec2(col, 1.)))*min(h, 2.5)*max(2.*rs, 1.);
    vec2 from = f + vec2(sin(f.y*.06/rs + u_now*2.)*h*.8*rs, sp);
    vec3 dr = texture(iChannel0, from/iResolution.xy).rgb;
    float keep = smoothstep(.04, .4, h);
    c = mix(c, mix(dr, c, .04), keep);
    float re = smoothstep(.003, .04, h)*(1. - keep);                  // cooling: tiles come back in the wrong places first
    if (re > 0.) { vec2 T = floor(L/48.); vec2 jump = (hash22(T + floor(u_now*7.)) - .5)*190.;
      c = mix(c, scn(u_a, s + jump), step(hash12(T + floor(u_now*4.)), re*min(1., h*4.))); }
  }
  if (u_mol > .002) {                                                  // microglobular: every pixel joins a bead
    float S = 20.*(1. + .6*vnoise(L/260. + u_now*.05));
    vec2 id = floor(L/S);
    float dm = 1e9, best = 1e9, br = 1.; vec2 bc = L;
    for (int j = -1; j <= 1; j++) for (int i = -1; i <= 1; i++) {
      vec2 cc = id + vec2(float(i), float(j)); float hs = hash12(cc + float(iScreen)*31.);
      float on = smoothstep(hs*.7, hs*.7 + .3, u_mol);
      if (on < .01) continue;
      vec2 ctr = (cc + .5 + .3*vec2(sin(u_now*(1. + hs)*2.1 + hs*20.), cos(u_now*(1.3 + hs)*1.7 + hs*9.)))*S;
      float r = S*(.18 + .42*hash12(cc + 3.))*on*(.85 + .3*u_mol);
      float d = length(L - ctr) - r;
      dm = smin(dm, d, 7.);
      if (d < best) { best = d; bc = ctr; br = max(r, .5); }
    }
    vec2 q = (L - bc)/br; float l = min(length(q), 1.);
    vec3 n = vec3(q/max(1., length(q)), sqrt(1. - l*l));
    vec3 lensC = scn(u_a, bc - q*br*2.6);                               // each bead holds the screen, small and upside down
    float sp = pow(max(dot(n, normalize(vec3(-.4, .6, .7))), 0.), 36.), fres = pow(1. - n.z, 2.);
    vec3 irid = spec(fres*.9 + u_now*.1);
    vec3 bead = lensC*(.6 + .55*n.z) + sp*1.2 + irid*fres*.7;
    float ao = smoothstep(0., 9., dm);
    vec3 goo = c*mix(.25, .85, ao)*(1. - .35*u_mol) + irid*.06*(1. - ao);
    c = mix(c, mix(bead, goo, smoothstep(-1., 1., dm)), smoothstep(0., .15, u_mol));
  }
  vec2 uv = C/CS, e2 = 3./CS;                                           // the strands of hyperreality, glowing
  vec3 sc = spec(st.w + st.z*.5)*.9 + .1;
  float halo = .25*(texture(u_str, uv + vec2(e2.x, 0.)).z + texture(u_str, uv - vec2(e2.x, 0.)).z + texture(u_str, uv + vec2(0., e2.y)).z + texture(u_str, uv - vec2(0., e2.y)).z);
  c = c*(1. - .3*st.z) + sc*(smoothstep(.12, .85, st.z)*1.25 + st.z*.3 + halo*.45) + smoothstep(.8, 1., st.z)*.5;
  if (u_ubik > .01) {                                                  // Ubik: safe when used as directed
    float g = step(1. - .035*u_ubik, hash12(floor(L/2.) + floor(u_now*20.)*vec2(7., 13.)));
    c = mix(c, vec3(1., .86, .45), g*.9) + vec3(1., .8, .35)*u_ubik*.10*vnoise(L*.03 + vec2(0., u_now*3.));
  }
  c += (hash12(f + fract(u_now*7.)*311.) - .5)*.04*(.4 + u_chaos);
  o = vec4(c, 1.);
}`;
const PRESENT = `void mainImage(out vec4 o, in vec2 f){ o = texelFetch(iChannel0, ivec2(gl_FragCoord.xy), 0); }`;

/* ---------- assemble the sketch's programs ---------- */
const glsl = { strands: STRANDS, comp: COMP, present: PRESENT, copy: PRESENT };
for (const sc of PH.list) {
  for (const [k, src] of Object.entries(sc.glsl || {})) glsl[`${sc.id}.${k}`] = k.startsWith('sim')
    ? `uniform vec4 u_p0, u_p1, u_p2, u_p3; uniform float u_t; uniform sampler2D u_ui, u_aux, u_aux2;\n${LIB}\n${src}`   // simulations: no UI helpers
    : `${SCENE_HEAD}${sc.common || ''}\n${src}`;
}
const KINDS = { cut: 0, regress: 1, tape: 2, open: 3, scramble: 4 };

/* ---------- the console's global state ---------- */
const store = { get(k, d) { try { const v = localStorage.getItem('phant.' + k); return v == null ? d : JSON.parse(v); } catch (e) { return d; } }, set(k, v) { try { localStorage.setItem('phant.' + k, JSON.stringify(v)); } catch (e) { /* private mode: no memory */ } } };
const G = PH.G = {
  boots: store.get('boots', 0), reality: 1, heat: [], presses: [], tear: 0, tearV: 0, mol: 0, ubik: 0, chaos: 0, now: 0,
  cur: null, prev: null, tr: 1, trKind: 0, trDur: 0.9, focus: [256, 192, 256, 192], seed: Math.random() * 10,
  strandsClear: true, lastSeg: null, hue: 0, freeze: false, motion: true,
  store,
  pool: [[new Layer(), new Layer()], [new Layer(), new Layer()]],
  status: new Layer(LW, 30, 2),
  go(id, kind = 'regress', o = {}) {
    const sc = PH.byId[id]; if (!sc) return;
    if (this.tr < 1 && !o.force) return;
    const pool = this.cur && this.cur.pool === 0 ? 1 : 0;
    const inst = o.resume || { sc, S: { t: 0 }, pool, L: this.pool[pool] };
    if (!o.resume) { inst.L[0].sig = inst.L[1].sig = null; sc.enter && sc.enter(inst.S, this, o); }
    this.prev = this.cur; this.cur = inst;
    this.trKind = KINDS[kind] ?? 1; this.tr = this.prev && this.trKind ? 0 : 1; this.trDur = o.dur || (kind === 'tape' ? 1.4 : kind === 'open' ? 1.1 : 0.9);
    if (o.focus) this.focus = o.focus;
    if (id === 'pause') this.freeze = true;
    if (PH.onScene) PH.onScene(sc, inst.S);
  },
  over(reason) { if (this.cur && this.cur.sc.id !== 'over') this.go('over', 'regress', { reason, force: true }); },
  splat(x, y, s) { this.heat.push([x, y, this.now, s]); if (this.heat.length > 12) this.heat.shift(); }
};

/* ---------- gamepad: east A, south B, north X, west Y, as on the console ---------- */
const PAD = { 1: 'A', 0: 'B', 3: 'X', 2: 'Y', 4: 'L', 5: 'R', 8: 'SELECT', 9: 'START', 12: 'UP', 13: 'DOWN', 14: 'LEFT', 15: 'RIGHT' };
let padMask = 0;
function pollPad(r) {
  const pads = navigator.getGamepads ? navigator.getGamepads() : []; let m = 0;
  for (const p of pads) { if (!p) continue; for (const [i, b] of Object.entries(PAD)) if (p.buttons[i] && p.buttons[i].pressed) m |= 1 << DS.BUTTONS.indexOf(b);
    if (p.axes && p.axes.length >= 2) { if (p.axes[0] < -.6) m |= 1 << 10; if (p.axes[0] > .6) m |= 1 << 11; if (p.axes[1] < -.6) m |= 1 << 8; if (p.axes[1] > .6) m |= 1 << 9; } }
  const ch = m ^ padMask; if (ch) DS.BUTTONS.forEach((b, i) => { if (ch & (1 << i)) DS.press(r, b, !!(m & (1 << i))); });
  padMask = m;
}

/* ---------- one frame ---------- */
function drawLayers(h, inst) {
  const { sc, S, L } = inst;
  for (let s = 0; s < 2; s++) {
    const fn = sc.ui && sc.ui[s ? 'bot' : 'top']; if (!fn) continue;
    const sig = (sc.sig ? sc.sig(S, G, s) : '') + '|' + PH.fontsReady;
    if (L[s].sig === sig && L[s].tex) continue;
    L[s].begin(); fn(L[s], S, G); L[s].upload(h.H); L[s].sig = sig;
  }
}
function drawStatus(h) {
  const d = new Date(), years = ['1992', '1959', '1974', 'AD 70', '1992', '1998'];
  const yr = years[Math.floor(G.now / 7) % years.length], hm = d.toTimeString().slice(0, 5);
  const MSG = ['YOU ARE THE ONE RUNNING DOWN', 'TRY UBIK', 'DO NOT RESUME', 'THIS IS NOT 1992', 'IT IS STILL WARM IN HERE'];
  const k = Math.floor(G.now / 37), slip = G.now % 37 > 33;   // every so often the system bar carries a message from the other side
  const name = slip ? MSG[k % MSG.length] : G.cur ? (G.cur.sc.status || G.cur.sc.name) : '';
  const pct = Math.round(G.reality * 100);
  const sig = [hm, yr, name, pct, slip, PH.fontsReady].join('|');   // day is gone: the year is what is out of joint
  if (G.status.sig === sig && G.status.tex) return;
  const U = G.status.begin();   // 256×192 is the target: nothing smaller than Silkscreen 16 survives it
  U.rect(0, 0, LW, 30, { ch: 'g' });
  U.text(hm, 10, 21, { size: 16 }); U.text(yr, 84, 21, { size: 16, a: .65 });
  U.text(name, slip ? 256 : 300, 21, { size: slip ? 8 : 16, align: 'center', a: .9, ch: slip ? 'b' : 'r', track: slip ? 1 : 0 });
  U.text(`${pct}%`, 432, 21, { size: 16, align: 'right' });
  U.frame(440, 8, 58, 15, { lw: 2 }); U.rect(444, 12, 50 * G.reality, 7, { ch: 'b' }); U.rect(498, 12, 4, 7, { ch: 'r' });
  G.status.upload(h.H); G.status.sig = sig;
}
function scenePass(h, inst, s, slot) {
  const { sc, S, L } = inst, [rw, rh] = h.res, gl = h.H.gl;
  const T = h.target(`scn${slot}${s}`, rw, rh);
  const tex = Object.assign({ u_ui: L[s].tex || h.H.white, u_mem: h.H.white, u_aux: h.H.white, u_aux2: h.H.white }, inst.tex || {});
  const uni = Object.assign({ u_t: S.t, u_in: Math.min(1, S.t / (sc.inDur || 1)), u_sel: S.sel || 0, u_life: G.reality, u_now: G.now }, inst.uni || {});
  gl.bindFramebuffer(gl.FRAMEBUFFER, T.fb); gl.viewport(0, 0, rw, rh); gl.disable(gl.BLEND);
  h.bind(h.prog(`${sc.id}.${s ? 'bot' : 'top'}`), { textures: tex, uniforms: uni }, s, [LW, LH, rw / LW]);
  gl.drawArrays(gl.TRIANGLES, 0, 3);
  return T;
}
function prepare(h, inst) {   // the scene's per-frame numbers and its own memories
  const { sc, S } = inst;
  inst.uni = sc.uni ? sc.uni(S, G) : {};
  inst.tex = sc.tex ? sc.tex(h, S, G, inst) : {};
}

const sketch = {
  id: 'phant', res: [256, 192], palette: 'Lattice dark', glsl, params: [],
  frame(h) {
    const dt = h.dt; G.now += dt; G.h = h;
    pollPad(h);
    if (!G.cur) G.go('boot', 'cut');
    /* ---- verbs ---- */
    const pressed = DS.BUTTONS.filter(b => h.pressed(b));
    G.presses = G.presses.filter(t => G.now - t < 1.5);
    const focus = G.cur.sc.focus ? G.cur.sc.focus(G.cur.S, G) : [256, 192];
    for (const b of pressed) {
      if (b === 'Y') continue;
      G.presses.push(G.now);
      const mash = Math.min(4, G.presses.length);
      const heat = [0, .05, .13, .26, .4][mash];   // one press barely drips; mashing melts
      if (G.cur.sc.id !== 'boot' || mash > 2) G.splat(focus[0] + (Math.random() - .5) * 120, focus[1] + (Math.random() - .5) * 80, heat);
    }
    const ub = h.held('Y');
    G.ubik = ub ? Math.min(1, G.ubik + dt * 4) : Math.max(0, G.ubik - dt * 1.5);
    const want = G.ubik > .2 ? 0 : (h.held('L') ? .55 : 0) + (h.held('R') ? .55 : 0);
    G.tearV += ((Math.min(1, want) - G.tear) * 40 - G.tearV * 7) * dt; G.tear = Math.max(0, G.tear + G.tearV * dt);
    G.mol = h.held('X') ? Math.min(1, G.mol + dt * .9) : Math.max(0, G.mol - dt * (.4 + G.ubik));
    if (G.ubik > 0) { for (const s of G.heat) s[3] *= Math.exp(-dt * 4 * G.ubik); G.reality = Math.min(1, G.reality + dt * .3 * G.ubik); }
    G.heat = G.heat.filter(s => G.now - s[2] < 9 && s[3] > .01);
    const heatSum = G.heat.reduce((a, s) => a + s[3] * Math.exp(-(G.now - s[2]) * .5), 0);
    G.chaos = Math.min(3, heatSum * .5 + G.tear * 2 + G.mol * 1.5);
    if (G.cur.sc.drain !== false && G.tr >= 1) G.reality = Math.max(0, G.reality - dt * (.0032 + .006 * G.chaos));
    if (G.reality <= 0) G.over('REALITY 0%');
    /* ---- strands: the stylus, on the bottom screen only ---- */
    const t = h.touch; let seg = [0, 0, 0, 0], down = 0;
    if (t.down) { const p = [t.x / 2, t.y / 2]; seg = G.lastSeg ? [G.lastSeg[0], G.lastSeg[1], p[0], p[1]] : [p[0], p[1], p[0], p[1]];
      if (!G.lastSeg || Math.hypot(p[0] - G.lastSeg[0], p[1] - G.lastSeg[1]) > .5) down = G.lastSeg ? 1 : 0; G.lastSeg = p; G.hue = (G.hue + dt * .15) % 1; }
    else G.lastSeg = null;
    /* ---- scenes ---- */
    const I = { press: b => h.pressed(b), held: b => h.held(b), touch: t, tap: t.tapped ? [t.x, LH - t.y] : null, dt };
    const inst = G.cur; inst.S.t += dt;
    if (inst.sc.cart && G.tr >= 1 && I.press('START')) G.go('pause', 'cut', { from: inst });          // every cartridge pauses the same way
    else if (inst.sc.cart && G.tr >= 1 && I.press('SELECT')) G.go('home', 'regress');
    else if (inst.sc.update) inst.sc.update(inst.S, G, I, dt);
    if (G.tr < 1) { G.tr = Math.min(1, G.tr + dt / G.trDur); if (G.prev) { G.prev.S.t += dt; if (G.prev.sc.idle) G.prev.sc.idle(G.prev.S, G, dt); } }
    const active = G.tr < 1 && G.prev ? [G.cur, G.prev] : [G.cur];
    for (const a of active) drawLayers(h, a);
    drawStatus(h);
    /* ---- the frozen frame for pause (Ubik's cold-pac) ---- */
    const [rw, rh] = h.res;
    if (G.freeze) { for (let s = 0; s < 2; s++) { const C = h.sims.get('comp' + s); if (C) h.pass('copy', h.target('frz' + s, rw, rh), { textures: { iChannel0: C.a } }); } G.freeze = false; }
    const strands = h.sim('strands', 'strands', 256, Math.round(CHT / 2), { everyFrame: true, float: true, linear: true,
      uniforms: { u_seg: seg, u_down: down, u_hue: G.hue, u_clear: G.strandsClear ? 1 : 0, u_fade: G.ubik } });
    G.strandsClear = false;
    for (const a of active) prepare(h, a);
    const A = [0, 1].map(s => scenePass(h, G.cur, s, G.cur.pool));
    const B = G.tr < 1 && G.prev ? [0, 1].map(s => scenePass(h, G.prev, s, G.prev.pool)) : A;
    const heatArr = new Float32Array(48); G.heat.forEach((s, i) => heatArr.set(s, i * 4));
    for (let s = 0; s < 2; s++) {
      const C = h.sim('comp' + s, 'comp', rw, rh, { everyFrame: true, screen: s,
        textures: { u_a: A[s], u_b: B[s], u_str: strands, u_status: G.status.tex },
        uniforms: { u_tr: G.tr, u_kind: G.trKind, u_tear: G.tear, u_mol: G.mol, u_ubik: G.ubik, u_chaos: G.chaos, u_life: G.reality, u_now: G.now,
          u_sb: G.cur.sc.statusBar === false ? 0 : 1, u_seed: G.seed, u_focus: G.focus, u_heat: heatArr } });
      h.screen(s ? 'bottom' : 'top', 'present', { textures: { iChannel0: C } });
    }
    if (PH.onFrame) PH.onFrame(G);
  }
};

/* ---------- the page ---------- */
PH.start = function (ui) {
  let H;
  try { H = DS.init(); } catch (e) { PH.onError && PH.onError(e); return null; }
  for (const sc of PH.list) DS.PALETTES[sc.id] = sc.pal;
  sketch.palette = 'boot';
  const r = DS.add(sketch, ui); r.visible = true; DS.focus = r; PH.run = r;
  DS.onError = (run, e) => PH.onError && PH.onError(e);
  const orig = sketch.frame; sketch.frame = (h) => { orig(h); if (G.cur && G.cur.sc.id !== G.palId) { G.palId = G.cur.sc.id; h.setPalette(G.palId); } };   // scenes can change outside a frame (the page's jump buttons)
  PH.fontsReady = 0;
  const faces = ['16px "Silkscreen"', '700 16px "Silkscreen"', '20px "VT323"', '24px "Jacquard 24"', '16px "Courier Prime"', '700 16px "Courier Prime"'];
  Promise.all(faces.map(f => document.fonts.load(f).catch(() => null))).then(() => { PH.fontsReady = 1; r.dirty = true; });
  DS.start();
  return r;
};
PH.press = (b, down) => PH.run && DS.press(PH.run, b, down);
PH.jump = (id) => { G.go(id, 'scramble', { force: true }); };
PH.setRes = (w, hgt) => { if (PH.run) { PH.run.res = [w, hgt]; PH.run.sims.forEach((S, k) => { if (k.startsWith('comp')) S.steps = 0; }); } };
PH.setMotion = (on) => { if (PH.run) { PH.run.running = on; PH.run.dirty = true; } };
})();
