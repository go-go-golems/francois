/* dshost.js — the browser twin of the DS shader contract (DSSHADE-001 guide §8, Phase 0).

   One shared WebGL2 context renders every screen of every sketch at that sketch's internal resolution,
   then each screen is copied nearest-neighbour into its own 2D canvas. That is the device's shape too:
   one GL context, two windows (FL:src/main.cpp:101-118, LD:src/platform/platform.cpp:64-76).

   A sketch is { glsl: {name: source}, params, res, frame(h) }. frame() is plain JS that calls the host:
     h.screen('top'|'bottom', prog, opts)   final pass of a screen → its canvas
     h.pass(prog, target, opts)             offscreen pass
     h.sim(key, prog, w, h, opts)           ping-pong simulation stepped at a fixed 60 Hz → texture
     h.typeField(key, spec)                 text as a field: r crisp, g soft, b halo (no_shade's typeAt)
     h.svg(key, svg, w, h)                  an SVG drawing as a texture (UI mock-ups stand in for Lattice frames)
   Every shader gets the same header: the uniforms of guide §8.1, no_shade's LIB, typeAt, prev().
   Textures are uploaded with y flipped, so every sampler in every shader is y-up, like gl_FragCoord. */
(function () {
'use strict';

const LW = 512, LH = 384, GAP = 92;                 // logical screen and hinge, in Lattice logical px
const BUDGET = 13.33e6;                              // Mali-G52 GPU-active cycles per 60 Hz frame (Falllinie PERF-009/010)
const BUTTONS = ['A', 'B', 'X', 'Y', 'L', 'R', 'START', 'SELECT', 'UP', 'DOWN', 'LEFT', 'RIGHT'];
const KEYMAP = { x: 'A', z: 'B', s: 'X', a: 'Y', q: 'L', w: 'R', Enter: 'START', Backspace: 'SELECT',
  ArrowUp: 'UP', ArrowDown: 'DOWN', ArrowLeft: 'LEFT', ArrowRight: 'RIGHT' };   // LD:src/platform/input.hpp:20-22

/* palettes: four tokens, as in no_shade; the Lattice ones are its real theme tokens (LD:src/gl/theme.cpp:11-31) */
const PALETTES = {
  'Lattice dark': { ground: '#121518', ink: '#E3E6E9', hot: '#F07158', cool: '#86A2F7' },
  'Lattice light': { ground: '#FAFAF8', ink: '#111417', hot: '#D23B1E', cool: '#1F4FD1' },
  'Blueprint': { ground: '#123A78', ink: '#F2F6FF', hot: '#FFC857', cool: '#A9C1EA' },
  'no_shade': { ground: '#161616', ink: '#F2F2F2', hot: '#FF3B1F', cool: '#3D5AFE' }
};

/* ---------- colour on the CPU: palettes glide in OKLab (no_shade NS.themeStep, NS:runtime.js:97) ---------- */
const lin = c => c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
const srgb = c => c <= 0.0031308 ? c * 12.92 : 1.055 * Math.pow(Math.max(c, 0), 1 / 2.4) - 0.055;
const hexRgb = h => { const n = parseInt(h.slice(1), 16); return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255]; };
const toLab = ([r, g, b]) => { r = lin(r); g = lin(g); b = lin(b);
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b), m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b), s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return [0.2104542553 * l + 0.7936177850 * m - 0.0040720468 * s, 1.9779984951 * l - 2.4285922050 * m + 0.4505937099 * s, 0.0259040371 * l + 0.7827717662 * m - 0.8086757660 * s]; };
const fromLab = ([L, a, b]) => { let l = L + 0.3963377774 * a + 0.2158037573 * b, m = L - 0.1055613458 * a - 0.0638541728 * b, s = L - 0.0894841775 * a - 1.2914855480 * b; l = l * l * l; m = m * m * m; s = s * s * s;
  return [4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s, -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s, -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s].map(c => Math.max(0, Math.min(1, srgb(c)))); };

/* ---------- GLSL: the contract header (guide §8.1) ---------- */
const LIB = `
float hash12(vec2 p){ vec3 p3 = fract(vec3(p.xyx)*.1031); p3 += dot(p3, p3.yzx + 33.33); return fract((p3.x + p3.y)*p3.z); }
vec2 hash22(vec2 p){ vec3 p3 = fract(vec3(p.xyx)*vec3(.1031, .1030, .0973)); p3 += dot(p3, p3.yzx + 33.33); return fract((p3.xx + p3.yz)*p3.zy); }
float vnoise(vec2 p){ vec2 i = floor(p), f = fract(p); vec2 u = f*f*(3. - 2.*f);
  return mix(mix(hash12(i), hash12(i + vec2(1., 0.)), u.x), mix(hash12(i + vec2(0., 1.)), hash12(i + vec2(1., 1.)), u.x), u.y); }
float fbm(vec2 p){ float v = 0., a = .5; mat2 m = mat2(1.6, 1.2, -1.2, 1.6); for (int i = 0; i < 5; i++){ v += a*vnoise(p); p = m*p; a *= .5; } return v; }
vec3 ns_lin(vec3 c){ return mix(c/12.92, pow((c + .055)/1.055, vec3(2.4)), step(.04045, c)); }
vec3 ns_srgb(vec3 c){ c = clamp(c, 0., 1.); return mix(c*12.92, 1.055*pow(c, vec3(1./2.4)) - .055, step(.0031308, c)); }
vec3 toOklab(vec3 s){ vec3 c = ns_lin(s);
  vec3 l = vec3(dot(c, vec3(.4122214708, .5363325363, .0514459929)), dot(c, vec3(.2119034982, .6806995451, .1073969566)), dot(c, vec3(.0883024619, .2817188376, .6299787005)));
  l = pow(max(l, 0.), vec3(1./3.));
  return vec3(dot(l, vec3(.2104542553, .7936177850, -.0040720468)), dot(l, vec3(1.9779984951, -2.4285922050, .4505937099)), dot(l, vec3(.0259040371, .7827717662, -.8086757660))); }
vec3 fromOklab(vec3 L){ vec3 l = vec3(L.x + .3963377774*L.y + .2158037573*L.z, L.x - .1055613458*L.y - .0638541728*L.z, L.x - .0894841775*L.y - 1.2914855480*L.z); l = l*l*l;
  return ns_srgb(vec3(dot(l, vec3(4.0767416621, -3.3077115913, .2309699292)), dot(l, vec3(-1.2684380046, 2.6097574011, -.3413193965)), dot(l, vec3(-.0041960863, -.7034186147, 1.7076147010)))); }
vec3 mixOk(vec3 a, vec3 b, float t){ return fromOklab(mix(toOklab(a), toOklab(b), clamp(t, 0., 1.))); }
`;
const header = (params, hasFloat) => `#version 300 es
precision highp float; precision highp int; precision highp sampler2D;
uniform vec3  iResolution;   // screens: (512, 384, internal scale); offscreen targets: (w, h, 1)
uniform float iTime;         // seconds, wrapped at 600
uniform int   iFrame;        // frame index (for a sim: its own step count)
uniform int   iScreen;       // 0 top (DSI-2), 1 bottom (DSI-1)
uniform vec4  iTouch;        // bottom screen only: xy logical px y-up, z down, w smoothed 0..1
uniform vec4  iPointer;      // this screen's pointer; on the top screen: mouse there, else the touch projected through the hinge
uniform ivec2 iButtons;      // x held, y pressed this frame; bits: A B X Y L R START SELECT UP DOWN LEFT RIGHT
uniform vec2  iTouchVel;     // lab extension: smoothed touch velocity, logical px/s
uniform vec3  u_ground, u_ink, u_hot, u_cool;
uniform float p_0, p_1, p_2, p_3, p_4, p_5;
uniform sampler2D iChannel0, iChannel1, iChannel2, u_type;
${params.map((p, i) => `#define p_${p[0]} p_${i}`).join('\n')}
#define HAS_FLOAT ${hasFloat ? 1 : 0}
const float DS_GAP = ${GAP}.;
const float PI = 3.14159265;
const int BTN_A = 0, BTN_B = 1, BTN_X = 2, BTN_Y = 3, BTN_L = 4, BTN_R = 5, BTN_START = 6, BTN_SELECT = 7, BTN_UP = 8, BTN_DOWN = 9, BTN_LEFT = 10, BTN_RIGHT = 11;
bool held(int b){ return (iButtons.x & (1 << b)) != 0; }
bool pressed(int b){ return (iButtons.y & (1 << b)) != 0; }
// both screens as one canvas, bottom screen at y 0..384, the hinge above it, the top screen above that
vec2 dsCanvas(vec2 f){ return iScreen == 0 ? f + vec2(0., ${LH}. + DS_GAP) : f; }
vec2 dsCanvasSize(){ return vec2(${LW}., ${2 * LH}. + DS_GAP); }
// text drawn by shaders (no_shade typeAt): x crisp coverage, y soft (crosses .5 at the edge), z wide halo
vec3 typeAt(vec2 f){ vec2 uv = f/vec2(${LW}., ${LH}.); if (any(lessThan(uv, vec2(0.))) || any(greaterThan(uv, vec2(1.)))) return vec3(0.); return texture(u_type, uv).rgb; }
// a simulation's last state (iChannel0), in its own cells
vec4 prev(vec2 f){ ivec2 s = textureSize(iChannel0, 0); return texelFetch(iChannel0, ivec2(mod(floor(f), vec2(s))), 0); }
vec4 prevClamp(vec2 f, vec4 outside){ ivec2 s = textureSize(iChannel0, 0), i = ivec2(floor(f)); if (any(lessThan(i, ivec2(0))) || any(greaterThanEqual(i, s))) return outside; return texelFetch(iChannel0, i, 0); }
vec4 prevSmooth(vec2 uv){ return texture(iChannel0, uv); }
` + LIB;
const FOOTER = `
out vec4 ns_out;
void main(){ mainImage(ns_out, gl_FragCoord.xy/iResolution.z); }`;
const VS = `#version 300 es
void main(){ vec2 p = vec2((gl_VertexID << 1) & 2, gl_VertexID & 2); gl_Position = vec4(p*2. - 1., 0., 1.); }`;   // FL:assets/shaders/post_vs.glsl

/* ---------- the shared GL context ---------- */
class Host {
  constructor(canvas) {
    this.canvas = canvas || document.createElement('canvas');   // the arcade passes its full-page stage canvas
    if (!canvas) { this.canvas.width = 1024; this.canvas.height = 768; }
    const gl = this.canvas.getContext('webgl2', { alpha: false, antialias: false, depth: false, stencil: false, preserveDrawingBuffer: true, premultipliedAlpha: false });
    if (!gl) throw new Error('This browser has no WebGL2, so the sketches cannot run.');
    this.gl = gl;
    this.hasFloat = !!gl.getExtension('EXT_color_buffer_float');
    this.vao = gl.createVertexArray();
    this.vs = this.shader(gl.VERTEX_SHADER, VS);
    this.progs = new Map();
    this.white = this.texFromPixels(1, 1, new Uint8Array([0, 0, 0, 255]));
  }
  shader(type, src) {
    const gl = this.gl, sh = gl.createShader(type);
    gl.shaderSource(sh, src); gl.compileShader(sh);
    if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) { const log = gl.getShaderInfoLog(sh); gl.deleteShader(sh); throw new Error(log); }
    return sh;
  }
  /** Assemble header + user source + footer, compile, link, and reflect every active uniform. Cached by source. */
  program(user, params) {
    const src = header(params, this.hasFloat) + '\n#line 1\n' + user + FOOTER;
    let P = this.progs.get(src); if (P) return P;
    const gl = this.gl, p = gl.createProgram();
    gl.attachShader(p, this.vs); gl.attachShader(p, this.shader(gl.FRAGMENT_SHADER, src)); gl.linkProgram(p);
    if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(p));
    const u = {}, n = gl.getProgramParameter(p, gl.ACTIVE_UNIFORMS);
    for (let i = 0; i < n; i++) { const info = gl.getActiveUniform(p, i), name = info.name.replace(/\[0\]$/, ''); u[name] = { loc: gl.getUniformLocation(p, info.name), type: info.type }; }
    P = { p, u }; this.progs.set(src, P); return P;
  }
  set(P, name, v) {
    const e = P.u[name]; if (!e) return;   // the driver strips unused uniforms; setting one is never an error
    const gl = this.gl, L = e.loc;
    switch (e.type) {
      case gl.FLOAT: gl.uniform1f(L, v); break;
      case gl.FLOAT_VEC2: gl.uniform2fv(L, v); break;
      case gl.FLOAT_VEC3: gl.uniform3fv(L, v); break;
      case gl.FLOAT_VEC4: gl.uniform4fv(L, v); break;
      case gl.INT: case gl.BOOL: gl.uniform1i(L, v); break;
      case gl.INT_VEC2: gl.uniform2iv(L, v); break;
      default: gl.uniform1i(L, v);
    }
  }
  texFromPixels(w, h, data) {
    const gl = this.gl, t = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, t);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, data);
    this.params(gl.LINEAR); return t;
  }
  texFromCanvas(c, t) {
    const gl = this.gl; t = t || gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, t);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, gl.RGBA, gl.UNSIGNED_BYTE, c);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, false);
    this.params(gl.LINEAR); return t;
  }
  params(filter, wrap) {
    const gl = this.gl;
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, wrap || gl.CLAMP_TO_EDGE); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, wrap || gl.CLAMP_TO_EDGE);
  }
  /** A render target. float: RGBA16F when the browser can render to it, else RGBA8 (the shader sees HAS_FLOAT). */
  target(w, h, o = {}) {
    const gl = this.gl, tex = gl.createTexture(), f = o.float && this.hasFloat;
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(gl.TEXTURE_2D, 0, f ? gl.RGBA16F : gl.RGBA8, w, h, 0, gl.RGBA, f ? gl.HALF_FLOAT : gl.UNSIGNED_BYTE, null);
    this.params(o.linear ? gl.LINEAR : gl.NEAREST, o.repeat ? gl.REPEAT : gl.CLAMP_TO_EDGE);
    const fb = gl.createFramebuffer();
    gl.bindFramebuffer(gl.FRAMEBUFFER, fb);
    gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0);
    gl.clearColor(0, 0, 0, 0); gl.clear(gl.COLOR_BUFFER_BIT);
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    return { tex, fb, w, h };
  }
}

/* ---------- one sketch on the page ---------- */
class SketchRun {
  constructor(host, sk, ui) {
    this.H = host; this.sk = sk; this.ui = ui;
    this.res = sk.res || [256, 192];
    this.palName = sk.palette || 'Lattice dark';
    this.pal = Object.fromEntries(Object.entries(PALETTES[this.palName]).map(([k, v]) => [k, toLab(hexRgb(v))]));
    this.rgb = {}; this.glide(0);
    this.params = Object.fromEntries((sk.params || []).map(p => [p[0], p[2]]));
    this.time = 0; this.frame = 0; this.dt = 0; this.running = true; this.dirty = true;
    this.touch = { x: 256, y: 192, down: 0, w: 0, vx: 0, vy: 0, tapped: false };
    this.top = { x: 256, y: 192, down: 0, active: false };
    this.heldMask = 0; this.latch = 0; this.pressedMask = 0;
    this.state = {}; this.scr = [null, null]; this.targets = new Map(); this.sims = new Map(); this.tex = new Map();
    this.error = null;
  }
  setPalette(name) { this.palName = name; this.dirty = true; }
  /** palette glide in OKLab; returns true while still moving */
  glide(dt) {
    const want = PALETTES[this.palName], k = dt > 0 ? 1 - Math.exp(-dt * 6) : 1; let moving = false;
    for (const n of ['ground', 'ink', 'hot', 'cool']) {
      const w = toLab(hexRgb(want[n])), c = this.pal[n].map((x, i) => x + (w[i] - x) * k);
      if (Math.hypot(...c.map((x, i) => x - w[i])) > 0.002) moving = true; else c.splice(0, 3, ...w);
      this.pal[n] = c; this.rgb[n] = fromLab(c);
    }
    return moving;
  }
  /* ----- the API a sketch's frame(h) uses ----- */
  param(n) { return this.params[n]; }
  pressed(b) { return !!(this.pressedMask & (1 << BUTTONS.indexOf(b))); }
  held(b) { return !!(this.heldMask & (1 << BUTTONS.indexOf(b))); }
  prog(name) { const src = this.sk.glsl[name]; if (src == null) throw new Error('no shader named ' + name); try { return this.H.program(src, this.sk.params || []); } catch (e) { throw new Error(name + '.glsl\n' + e.message); } }
  pointerFor(screen) {
    const t = this.touch;
    if (screen === 1) return [t.x, t.y, t.down, t.w];
    if (this.top.active) return [this.top.x, this.top.y, this.top.down, 1];
    return [t.x, t.y - (LH + GAP), t.down, t.w];   // the finger seen through the hinge
  }
  bind(P, o, screen, res, simFrame) {
    const H = this.H, gl = H.gl;
    gl.useProgram(P.p);
    H.set(P, 'iResolution', res); H.set(P, 'iTime', this.time % 600); H.set(P, 'iFrame', simFrame ?? this.frame);
    H.set(P, 'iScreen', screen); H.set(P, 'iTouch', [this.touch.x, this.touch.y, this.touch.down, this.touch.w]);
    H.set(P, 'iPointer', this.pointerFor(screen)); H.set(P, 'iButtons', [this.heldMask, this.pressedMask]);
    H.set(P, 'iTouchVel', [this.touch.vx, this.touch.vy]);
    for (const n of ['ground', 'ink', 'hot', 'cool']) H.set(P, 'u_' + n, this.rgb[n]);
    (this.sk.params || []).forEach((p, i) => H.set(P, 'p_' + i, +this.params[p[0]]));
    for (const [k, v] of Object.entries(o.uniforms || {})) H.set(P, k, v);
    let unit = 0;
    for (const name of ['iChannel0', 'iChannel1', 'iChannel2', 'u_type']) { gl.activeTexture(gl.TEXTURE0 + unit); gl.bindTexture(gl.TEXTURE_2D, H.white); H.set(P, name, unit); unit++; }
    unit = 0;
    for (const [k, t] of Object.entries(o.textures || {})) { const u = ['iChannel0', 'iChannel1', 'iChannel2', 'u_type'].indexOf(k); const at = u >= 0 ? u : 4 + unit++; gl.activeTexture(gl.TEXTURE0 + at); gl.bindTexture(gl.TEXTURE_2D, t && t.tex ? t.tex : t); H.set(P, k, at); }
    gl.bindVertexArray(H.vao);
  }
  /** The final pass of one screen, drawn at the internal resolution, then copied to the screen's canvas. */
  screen(which, name, o = {}) {
    const H = this.H, gl = H.gl, s = which === 'top' ? 0 : 1, [rw, rh] = this.res;
    if (this.ui.offscreen) {   // the arcade: keep the screen as a mipmapped texture; the stage composites it
      let T = this.scr[s];
      if (!T || T.w !== rw || T.h !== rh) { T = this.scr[s] = H.target(rw, rh); gl.bindTexture(gl.TEXTURE_2D, T.tex); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR); gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR); }
      gl.bindFramebuffer(gl.FRAMEBUFFER, T.fb); gl.viewport(0, 0, rw, rh); gl.disable(gl.BLEND);
      this.bind(this.prog(name), o, s, [LW, LH, rw / LW]);
      gl.drawArrays(gl.TRIANGLES, 0, 3);
      gl.bindTexture(gl.TEXTURE_2D, T.tex); gl.generateMipmap(gl.TEXTURE_2D);
      return;
    }
    gl.bindFramebuffer(gl.FRAMEBUFFER, null); gl.viewport(0, 0, rw, rh); gl.disable(gl.BLEND);
    this.bind(this.prog(name), o, s, [LW, LH, rw / LW]);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
    const c = this.ui.screens[s], ctx = c.getContext('2d');
    ctx.imageSmoothingEnabled = false;   // nearest-neighbour upscale, like Falllinie's copy_fs
    ctx.drawImage(H.canvas, 0, H.canvas.height - rh, rw, rh, 0, 0, c.width, c.height);
  }
  target(key, w, h, o) { let T = this.targets.get(key); if (!T || T.w !== w || T.h !== h) { T = this.H.target(w, h, o); this.targets.set(key, T); } return T; }
  /** An offscreen pass into target T (iResolution = its size, f = its pixels). */
  pass(name, T, o = {}) {
    const gl = this.H.gl;
    gl.bindFramebuffer(gl.FRAMEBUFFER, T.fb); gl.viewport(0, 0, T.w, T.h); gl.disable(gl.BLEND);
    this.bind(this.prog(name), o, o.screen ?? 1, [T.w, T.h, 1], o.frame);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }
  /** A ping-pong simulation, stepped at a fixed 60 Hz whatever the display rate (no_shade toys, NS:runtime.js:2562). */
  sim(key, name, w, h, o = {}) {
    let S = this.sims.get(key);
    if (!S || S.w !== w || S.h !== h) { S = { a: this.H.target(w, h, o), b: this.H.target(w, h, o), w, h, steps: 0, acc: 0 }; this.sims.set(key, S); }
    if (this.running || S.steps === 0) {
      S.acc = Math.min(2 / 60, S.acc + (S.steps === 0 ? 1 / 60 : this.dt));
      const ticks = Math.min(2, Math.floor(S.acc * 60 + 0.08)); S.acc = Math.max(0, S.acc - ticks / 60);
      const n = o.everyFrame ? 1 : ticks * (o.steps || 1);   // everyFrame: exactly one step per drawn frame
      for (let i = 0; i < n; i++) {
        this.pass(name, S.b, Object.assign({}, o, { frame: S.steps, textures: Object.assign({}, o.textures, { iChannel0: S.a }) }));
        const t = S.a; S.a = S.b; S.b = t; S.steps++;
      }
    }
    return S.a;
  }
  resetSims() { for (const S of this.sims.values()) S.steps = 0; this.dirty = true; }
  /** Text as a field, the way no_shade's textTex does it (NS:runtime.js:2085): crisp coverage in red,
      a soft copy in green and a wide halo in blue, drawn off-canvas so only their shadows land. */
  typeField(key, spec) {
    const sig = JSON.stringify(spec); let T = this.tex.get(key);
    if (T && T.sig === sig) return T.tex;
    const k = 2, c = document.createElement('canvas'); c.width = LW * k; c.height = LH * k;
    const x = c.getContext('2d'), OFF = 20000, px = spec.size * k;
    x.fillStyle = '#000'; x.fillRect(0, 0, c.width, c.height);
    x.font = `${spec.weight || 800} ${px}px ${spec.font || 'Archivo, Arial Black, sans-serif'}`;
    x.textAlign = spec.align || 'center'; x.textBaseline = 'middle';
    if ('letterSpacing' in x) x.letterSpacing = ((spec.track || 0) * px) + 'px';
    const lines = String(spec.text).split('\n'), lh = px * (spec.lh || 1.05);
    const at = (i) => [(spec.x ?? 256) * k, (LH - (spec.y ?? 192)) * k + (i - (lines.length - 1) / 2) * lh];
    x.globalCompositeOperation = 'lighter';
    const shadow = (col, blur) => { x.save(); x.shadowColor = col; x.shadowBlur = blur; x.shadowOffsetX = OFF; x.fillStyle = col; lines.forEach((l, i) => { const [a, b] = at(i); x.fillText(l, a - OFF, b); }); x.restore(); };
    shadow('#00ff00', Math.max(3, px * 0.1));
    shadow('#0000ff', px * 0.55);
    x.fillStyle = '#ff0000'; lines.forEach((l, i) => { const [a, b] = at(i); x.fillText(l, a, b); });
    const tex = this.H.texFromCanvas(c, T && T.tex);
    this.tex.set(key, { sig, tex }); return tex;
  }
  /** An SVG drawing as a texture. Until it has loaded, a flat placeholder. */
  svg(key, svg, w = 1024, h = 768) {
    let T = this.tex.get(key);
    if (T && T.svg === svg) return T.tex;
    const tex = T ? T.tex : this.H.texFromPixels(1, 1, new Uint8Array([128, 128, 128, 255]));
    T = { svg, tex }; this.tex.set(key, T);
    const img = new Image();
    img.onload = () => { try { const c = document.createElement('canvas'); c.width = w; c.height = h; c.getContext('2d').drawImage(img, 0, 0, w, h); this.H.texFromCanvas(c, tex); this.dirty = true; } catch (e) { console.warn('svg texture', e); } };
    img.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg);
    return tex;
  }
  budget() { const px = this.res[0] * this.res[1] * 2; return { px, cpp: BUDGET / px }; }
}

/* ---------- the page loop: only sketches on screen run; one has the keyboard ---------- */
const DS = { LW, LH, GAP, BUDGET, PALETTES, BUTTONS, KEYMAP, runs: [], focus: null, host: null };
DS.init = (canvas) => { if (!DS.host) DS.host = new Host(canvas); return DS.host; };
DS.add = (sk, ui) => { const r = new SketchRun(DS.init(), sk, ui); r.visible = false; DS.runs.push(r); wireInput(r); return r; };

function wireInput(r) {
  const [top, bot] = r.ui.screens;
  const pos = (e, c) => { const b = c.getBoundingClientRect(); return [(e.clientX - b.left) / b.width * LW, (1 - (e.clientY - b.top) / b.height) * LH]; };
  let last = null;
  bot.addEventListener('pointerdown', e => { bot.setPointerCapture(e.pointerId); const [x, y] = pos(e, bot); Object.assign(r.touch, { x, y, down: 1, tapped: true }); last = [x, y, performance.now()]; DS.focus = r; r.dirty = true; e.preventDefault(); });
  bot.addEventListener('pointermove', e => { if (!r.touch.down) return; const [x, y] = pos(e, bot), now = performance.now();
    if (last) { const dt = Math.max(1, now - last[2]) / 1000; r.touch.vx = r.touch.vx * 0.5 + (x - last[0]) / dt * 0.5; r.touch.vy = r.touch.vy * 0.5 + (y - last[1]) / dt * 0.5; }
    last = [x, y, now]; Object.assign(r.touch, { x, y }); r.dirty = true; });
  const up = () => { r.touch.down = 0; r.touch.vx = r.touch.vy = 0; r.dirty = true; };
  bot.addEventListener('pointerup', up); bot.addEventListener('pointercancel', up);
  top.addEventListener('pointermove', e => { const [x, y] = pos(e, top); Object.assign(r.top, { x, y, active: true }); r.dirty = true; });
  top.addEventListener('pointerdown', e => { top.setPointerCapture(e.pointerId); const [x, y] = pos(e, top); Object.assign(r.top, { x, y, down: 1, active: true }); DS.focus = r; r.dirty = true; e.preventDefault(); });
  top.addEventListener('pointerup', () => { r.top.down = 0; });
  top.addEventListener('pointerleave', () => { if (!r.top.down) r.top.active = false; r.dirty = true; });
  r.ui.card.addEventListener('pointerdown', () => { DS.focus = r; DS.onFocus && DS.onFocus(r); });
}
DS.press = (r, b, down) => { const bit = 1 << BUTTONS.indexOf(b); if (down) { r.heldMask |= bit; r.latch |= bit; } else r.heldMask &= ~bit; r.dirty = true; };
addEventListener('keydown', e => { const b = KEYMAP[e.key]; const r = DS.focus; if (!b || !r || !r.visible || /INPUT|SELECT|TEXTAREA/.test(document.activeElement && document.activeElement.tagName)) return; if (!e.repeat) DS.press(r, b, true); e.preventDefault(); });
addEventListener('keyup', e => { const b = KEYMAP[e.key]; if (b && DS.focus) DS.press(DS.focus, b, false); });

let lastT = performance.now();
function tick(now) {
  const dt = Math.min(0.05, (now - lastT) / 1000); lastT = now;
  for (const r of DS.runs) {
    if (!r.visible || r.error) continue;
    const moving = r.glide(dt);
    r.touch.w += (r.touch.down - r.touch.w) * (1 - Math.exp(-dt * 12));
    if (!r.running && !r.dirty && !moving && Math.abs(r.touch.down - r.touch.w) < 0.01) continue;
    r.dt = r.running ? dt : 0;
    if (r.running) { r.time += dt; r.frame++; }
    r.pressedMask = r.latch; r.latch = 0;
    try { r.sk.frame(r); } catch (e) { r.error = e; DS.onError && DS.onError(r, e); console.error(r.sk.id, e); }
    r.touch.tapped = false; r.pressedMask = 0; r.dirty = false;
  }
  if (DS.after) { try { DS.after(dt, now); } catch (e) { console.error(e); } }   // the arcade's stage composites here
  requestAnimationFrame(tick);
}
DS.start = () => requestAnimationFrame(tick);
window.DS = DS;
})();
