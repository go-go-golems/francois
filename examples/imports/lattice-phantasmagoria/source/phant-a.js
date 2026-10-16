/* phant-a.js — the console's own furniture: boot, the cartridge menu, pause and game over. */
(function () {
'use strict';
const PH = window.PHANT = window.PHANT || { list: [], byId: {} };
const scene = (s) => { PH.list.push(s); PH.byId[s.id] = s; return s; };
const LH = 384;
const clamp = (x, a, b) => Math.max(a, Math.min(b, x));

/* the six cartridges, each a console element seen through one of Dick's books */
PH.CARTS = [
  { id: 'halflife', name: 'HALF-LIFE', label: ['HALF-LIFE'], slip: 'SAVE FILES', book: 'Ubik', c: [-.8, .156],
    desc: 'Three save files in cold-pac. They are running down, back toward 1939.' },
  { id: 'kipple', name: 'KIPPLE', label: ['KIPPLE'], slip: 'INVENTORY', book: 'Do Androids Dream of Electric Sheep?', c: [.285, .01],
    desc: 'Twenty-four slots for your things. Kipple drives out nonkipple.' },
  { id: 'empathy', name: 'EMPATHY BOX', label: ['EMPATHY', 'BOX'], slip: 'HEALTH BAR', book: 'Do Androids Dream of Electric Sheep?', c: [-.4, .6],
    desc: 'Grip the handles. Everyone’s health bar is one health bar.' },
  { id: 'scramble', name: 'SCRAMBLE SUIT', label: ['SCRAMBLE', 'SUIT'], slip: 'DIALOGUE BOX', book: 'A Scanner Darkly', c: [-.70176, -.3842],
    desc: 'Somebody is talking to you. Their face is everybody’s face.' },
  { id: 'beam', name: 'PINK BEAM', label: ['PINK', 'BEAM'], slip: 'WORLD MAP', book: 'VALIS', c: [-.835, -.2321],
    desc: 'A map in two times at once. The Empire never ended.' },
  { id: 'layout', name: 'LAYOUT', label: ['LAYOUT'], slip: 'THE CONSOLE', book: 'The Three Stigmata of Palmer Eldritch', c: [.355, .355],
    desc: 'A little room with a little console in it. Chew to go in.' }
];
const ANG = [-.06, .05, -.03, .07, -.05, .04];   // each slip of paper lies a little crooked (the shader knows the same angles)
const tileXY = i => [24 + (i % 3) * 158, 50 + Math.floor(i / 3) * 156];   // top-left, y down

/* ═══════════════════════ BOOT · The Electric Ant ═══════════════════════ */
const PHRASE = 'LATTICE REALITY SUPPLY 1992.3 * BELIEVE WHAT IS READ * ';
const WARN = 'Before you play, make sure you are real. Your world is read from a punched tape that runs through this console. Touch the tape to change what you are.';
const TAPE = `
// the reality tape: eight tracks of holes, running down through both screens and the hinge
float tape(vec2 c, out float hole){
  const float X0 = 446., W = 46.;
  hole = 0.;
  if (c.x < X0 || c.x > X0 + W) return 0.;
  float y = c.y + u_p0.y, lx = c.x - X0 - 3.;
  float row = floor(y/9.), tr = floor(lx/5.);
  vec2 q = vec2(mod(lx, 5.) - 2.5, mod(y, 9.) - 4.5);
  float bit = (tr >= 0. && tr < 8.) ? texelFetch(u_mem, ivec2(int(tr), int(mod(row, 128.))), 0).r : 0.;
  hole = step(length(q), 1.8)*step(.5, bit);
  hole = max(hole, step(length(vec2(lx - 15., q.y)), .9));
  return 1.;
}
vec3 tapeColour(vec2 c, vec3 col){
  float hole, on = tape(c, hole);
  if (on < .5) return col*(1. - .5*exp(-min(abs(c.x - 446.), abs(c.x - 492.))/3.));
  vec3 paper = vec3(.93, .86, .66)*(.86 + .1*vnoise(c*vec2(.5, .05)) + .04*step(.5, fract(c.x*.5)));
  return mix(paper, u_ground*.35, hole);
}`;
scene({
  id: 'boot', name: 'BOOT', status: 'BOOT', element: 'Boot screen and the health-and-safety warning', book: 'The Electric Ant',
  pal: { ground: '#0B0A10', ink: '#ECE6D6', hot: '#FF3D8B', cool: '#38E2B4' },
  statusBar: false, drain: false, inDur: 3.2,
  hint: ['The logo crystallises out of static. Every boot is a little different.', 'Touch the paper tape on the right to punch a hole. The hole flips one bit of the logo.', 'Touch anywhere else, or press A, to start.'],
  enter(S, G) {
    G.boots++; G.store.set('boots', G.boots); G.reality = 1; G.heat = []; G.mol = 0; G.strandsClear = true;
    S.word = ['LATTICE', 'LATTICE', 'LATTICF', 'KIPPLE'][G.boots % 4];
    S.scroll = 0; S.glitch = 0; S.hue = Math.random() * 6;
    S.bits = new Uint8Array(8 * 128 * 4);
    for (let r = 0; r < 128; r++) { const ch = PHRASE.charCodeAt(r % PHRASE.length); for (let t = 0; t < 8; t++) { const i = (r * 8 + t) * 4; S.bits[i] = ((ch >> (7 - t)) & 1) * 255; S.bits[i + 3] = 255; } }
    S.tapeDirty = true;
  },
  update(S, G, I, dt) {
    S.scroll += dt * 26; S.glitch = Math.max(0, S.glitch - dt * 2.5);
    const tap = I.tap;
    if (tap && tap[0] >= 446 && tap[0] <= 492) {
      const y = (LH - tap[1]) + S.scroll, row = ((Math.floor(y / 9) % 128) + 128) % 128, tr = clamp(Math.floor((tap[0] - 449) / 5), 0, 7);
      const i = (row * 8 + tr) * 4; S.bits[i] = S.bits[i] ? 0 : 255; S.tapeDirty = true; S.glitch = 1;
      const k = Math.floor(Math.random() * S.word.length), code = S.word.charCodeAt(k) ^ (1 << (7 - tr));
      S.word = S.word.slice(0, k) + (code >= 33 && code <= 126 ? String.fromCharCode(code) : '?') + S.word.slice(k + 1);
      G.splat(469, LH - tap[1], .5);
    } else if ((tap && S.t > 1.2) || I.press('A') || I.press('START')) G.go('home', 'tape');
  },
  focus: () => [220, 270],
  sig: (S, G, s) => s ? `${Math.min(WARN.length, Math.floor(Math.max(0, S.t - .5) * 55))}` : S.word,
  ui: {
    top(U, S, G) {
      const size = Math.min(118, 100 * 380 / Math.max(1, U.measure(S.word, { f: 'jac', size: 100 })));
      U.text(S.word, 220, 196 + size * .3, { f: 'jac', size, align: 'center' });
      U.text(`REALITY TAPE 1992.3  REEL ${String(G.boots).padStart(3, '0')}`, 220, 300, { size: 16, align: 'center', ch: 'g' });
    },
    bot(U, S) {
      U.text('!', 220, 136, { size: 48, w: 700, align: 'center', ch: 'b' });
      U.text('WARNING - REALITY AND SAFETY', 220, 196, { size: 16, align: 'center' });
      U.wrap(WARN.slice(0, Math.floor(Math.max(0, S.t - .5) * 55)), 26, 224, 396, { f: 'vt', size: 22, lh: 21 });
      U.text('TOUCH THE SCREEN TO CONTINUE', 220, 370, { size: 16, align: 'center', ch: 'g' });
    }
  },
  tex(h, S) {
    const gl = h.H.gl;
    if (!S.tape) S.tape = h.H.texFromPixels(8, 128, S.bits);
    else if (S.tapeDirty) { gl.bindTexture(gl.TEXTURE_2D, S.tape); gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, 8, 128, 0, gl.RGBA, gl.UNSIGNED_BYTE, S.bits); }
    S.tapeDirty = false; return { u_mem: S.tape };
  },
  uni: (S) => ({ u_p0: [S.hue, S.scroll, S.glitch, S.t > 3.4 ? 1 : 0] }),
  common: TAPE,
  glsl: {
    top: `
void mainImage(out vec4 o, in vec2 f){
  vec2 c = dsCanvas(f); float t = u_t, form = u_in;
  vec2 g = f + vec2((hash12(vec2(floor(f.y/6.), floor(t*20.))) - .5)*46.*u_p0.z, 0.);   // a punch makes the rows slip
  vec3 col = u_ground*(.7 + .3*f.y/384.);
  vec3 U = ui(g), Us = uiL(g, 2.5);
  vec2 ip = ipx(f);
  float n = hash12(ip + floor(t*24.)*vec2(17., 31.));
  col = mix(col, u_ink*.7, step(1. - (Us.r*1.6 + .05)*(1. - form)*.95, n)*.8);          // static, densest where the letters will be
  float front = form*1.4 - fbm(f*.025 + u_p0.x)*.9 - .1;
  float logo = smoothstep(.3, .6, U.r)*step(hash12(ip + 7.), front);                   // the logo joins pixel by pixel behind a noisy front
  vec2 sub = mod(g*5. + vec2(t*30., 0.), LS);
  float inner = smoothstep(.3, .6, textureLod(u_ui, sub/LS, UILOD() + 1.).r);         // inside, the logo is made of the logo
  vec3 lc = mixOk(u_hot, u_cool, .5 + .5*sin(f.x*.02 + t*1.3 + u_p0.x));
  col = mix(col, mix(lc*.5, mixOk(u_ink, lc, .3), inner), logo);
  col += lc*Us.r*.32*form;
  col = mix(col, u_ink*.8, smoothstep(.3, .6, U.g)*smoothstep(.85, 1., form));
  col = tapeColour(c, col);
  if (c.x > 446. && c.x < 492.) col = mix(col, u_hot, exp(-f.y/3.)*.9);              // the read head, right at the hinge
  o = vec4(col, 1.);
}`,
    bot: `
float sdTri(vec2 p, float r){ const float k = 1.7320508; p.x = abs(p.x) - r; p.y = p.y + r/k; if (p.x + k*p.y > 0.) p = vec2(p.x - k*p.y, -k*p.x - p.y)/2.; p.x -= clamp(p.x, -2.*r, 0.); return -length(p)*sign(p.y); }
// the warning triangle is a Sierpinski gasket, zooming forever into its own apex
float gasket(vec2 p, float z){
  const vec2 A = vec2(0., 1.1547), B = vec2(-1., -.57735), C = vec2(1., -.57735);
  p = A + (p - A)*exp2(-z);
  float s = 1.;
  for (int i = 0; i < 8; i++) { vec2 v = A; float d = dot(p - A, p - A), db = dot(p - B, p - B), dc = dot(p - C, p - C);
    if (db < d) { v = B; d = db; } if (dc < d) v = C; p = 2.*p - v; s *= 2.; }
  return sdTri(p, 1.)/s;
}
void mainImage(out vec4 o, in vec2 f){
  float t = u_t;
  vec3 col = u_ground*(.8 + .2*f.y/384.);
  vec2 p = (f - vec2(220., 270.))/86.6;
  float outer = sdTri(p, 1.)*86.6;
  float gk = gasket(p, fract(t*.18))*86.6;
  vec3 warm = mixOk(u_hot, u_ink, .15 + .25*sin(length(p)*6. - t*2.));
  col = mix(col, warm, aa(max(gk, outer)));
  col = mix(col, u_ink, aa(abs(outer) - 1.));
  col += u_hot*exp(-max(outer, 0.)/10.)*.12;
  float drift = smoothstep(5., 25., t)*7.;
  vec2 g = f + curl(f*.012 + t*.07)*drift;                                             // the paragraph slowly forgets where it was printed
  vec3 U = ui(g);
  col = mix(col, u_ground, smoothstep(.3, .6, ui(f).b));
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  float blink = step(.5, fract(t*.9))*u_p0.w;
  col = mix(col, u_cool, smoothstep(.3, .6, U.g)*blink);
  col = tapeColour(f, col);
  o = vec4(col, 1.);
}`
  }
});

/* ═══════════════════════ HOME · Time Out of Joint ═══════════════════════ */
scene({
  id: 'home', name: 'CARTRIDGES', status: 'CARTRIDGES', element: 'Home menu: icons, cursor, clock', book: 'Time Out of Joint',
  pal: { ground: '#17131D', ink: '#F2E7CF', hot: '#E8452E', cool: '#79C6D4' },
  hint: ['Move with the arrows or touch a cartridge. The cursor is a gravity well.', 'Cartridges you ignore turn into slips of paper with their real name typed on them.', 'A or a second touch starts one. SELECT wakes up the whole screen.'],
  enter(S, G, o) {
    S.sel = o.sel ?? (PH.lastCart || 0); S.cur = null; S.pres = (PH.homePres || [1, 1, 1, 1, 1, 1]).slice(); S.slip = 0; S.slipT = 0; S.slipOn = 0;
    S.c = PH.CARTS[S.sel].c.slice(); S.lens = 0;
  },
  update(S, G, I, dt) {
    let sel = S.sel; const col = sel % 3, row = Math.floor(sel / 3);
    if (I.press('LEFT')) sel = row * 3 + (col + 2) % 3; if (I.press('RIGHT')) sel = row * 3 + (col + 1) % 3;
    if (I.press('UP') || I.press('DOWN')) sel = (sel + 3) % 6;
    let go = I.press('A') || I.press('START');
    if (I.tap) for (let i = 0; i < 6; i++) { const [x, y] = tileXY(i); if (I.tap[0] >= x && I.tap[0] <= x + 148 && I.tap[1] >= y && I.tap[1] <= y + 146) { if (i === S.sel) go = true; sel = i; } }
    S.sel = sel; PH.lastCart = sel;
    const [tx, ty] = tileXY(sel), want = [tx + 74, LH - (ty + 73)];
    if (!S.cur) S.cur = want.slice();
    S.cur[0] += (want[0] - S.cur[0]) * (1 - Math.exp(-dt * 10)); S.cur[1] += (want[1] - S.cur[1]) * (1 - Math.exp(-dt * 10));
    S.lens = Math.min(1, S.lens + dt * 2);
    const c = PH.CARTS[sel].c; S.c[0] += (c[0] - S.c[0]) * (1 - Math.exp(-dt * 4)); S.c[1] += (c[1] - S.c[1]) * (1 - Math.exp(-dt * 4));
    for (let i = 0; i < 6; i++) S.pres[i] = i === sel ? Math.min(1, S.pres[i] + dt * 1.5) : Math.max(0, S.pres[i] - dt * .034);
    PH.homePres = S.pres;
    S.slipT += dt;
    if (I.press('SELECT') || S.slipT > 26) { S.slipOn = 2.6; S.slipT = 0; }
    S.slipOn = Math.max(0, S.slipOn - dt);
    S.slip += ((S.slipOn > 0 ? 1 : 0) - S.slip) * (1 - Math.exp(-dt * 3));
    if (go) G.go(PH.CARTS[sel].id, 'open', { focus: [256, 192, want[0], want[1]] });
  },
  focus: (S) => S.cur ? [S.cur[0], S.cur[1]] : [256, 192],
  sig: (S, G, s) => s ? 'tiles' : `${S.sel}|${Math.floor(G.now / 7)}|${new Date().getMinutes()}`,
  ui: {
    top(U, S, G) {
      const cart = PH.CARTS[S.sel];
      U.text(cart.name, 244, 92, { f: 'jac', size: 44 });
      U.wrap(cart.desc, 244, 124, 248, { f: 'vt', size: 22, lh: 21 });
      U.text(`after ${cart.book}`, 244, 124 + 21 * U.lines(cart.desc, 248, { f: 'vt', size: 22 }).length + 6, { f: 'vt', size: 18, a: .7 });
      const d = new Date(), years = ['1959', '1959', '1998', '1959', '1992', '1959'];
      U.text(d.toTimeString().slice(0, 5), 492, 336, { size: 48, align: 'right', ch: 'g' });
      U.text(`${d.toLocaleDateString('en-GB', { weekday: 'short', day: '2-digit', month: 'short' }).toUpperCase().replace(',', '')} ${years[Math.floor(G.now / 7) % 6]}`, 492, 362, { size: 16, align: 'right' });
      U.text('PREVIEW', 124, 270, { size: 8, align: 'center', a: .7, track: 2 });
      U.text('A INSERT', 24, 300, { size: 16 }); U.text('SELECT WAKE UP', 24, 322, { size: 16, a: .7 });
      U.text('HOME MENU', 256, 222, { f: 'cou', w: 700, size: 62, align: 'center', ch: 'b', rot: .05 });
    },
    bot(U) {
      U.text('CARTRIDGES', 24, 34, { size: 16 }); U.text('6 INSERTED', 488, 34, { size: 8, align: 'right', a: .7, track: 2 });
      PH.CARTS.forEach((c, i) => {
        const [x, y] = tileXY(i);
        U.rect(x, y, 148, 146, { ch: 'g', r: 7 });
        c.label.forEach((l, k) => U.text(l, x + 74, y + 118 + k * 18 - (c.label.length - 1) * 9, { size: 16, align: 'center' }));
        U.text(c.slip, x + 74, y + 79, { f: 'cou', w: 700, size: 17, align: 'center', ch: 'b', rot: ANG[i] });
      });
      U.text('TOUCH TWICE OR PRESS A TO START', 256, 376, { size: 8, align: 'center', a: .6, track: 1 });
    }
  },
  uni: (S) => ({ u_sel: S.sel, u_p0: [S.cur ? S.cur[0] : 256, S.cur ? S.cur[1] : 192, S.lens, 0], u_p1: S.pres.slice(0, 4), u_p2: [S.pres[4], S.pres[5], S.c[0], S.c[1]], u_p3: [S.slip, 0, 0, 0] }),
  common: `
const float ANG[6] = float[6](-.06, .05, -.03, .07, -.05, .04);
vec2 tileC(int i){ float col = float(i - 3*(i/3)), row = float(i/3); return vec2(24. + col*158. + 74., 384. - (50. + row*156. + 73.)); }
vec3 paperOf(vec2 g){ return mixOk(u_ink, vec3(.95, .9, .78), .6)*(.88 + .12*vnoise(g*.4)); }`,
  glsl: {
    top: `
void mainImage(out vec4 o, in vec2 f){
  vec3 col = u_ground*(.85 + .15*f.y/384.);
  vec2 w = mod(f + vec2(0., u_t*3.), 72.) - 36.; float ang = atan(w.y, w.x);
  col += u_cool*.035*(1. - smoothstep(0., 1.5, abs(length(w) - 6. - 4.*step(.5, fract(ang*8./6.2832)))));   // 1959 wallpaper
  vec2 pc = vec2(124., 384. - 152.), q = (f - pc)/100.;
  if (max(abs(q.x), abs(q.y)) < 1.) {                                       // the preview: the cartridge's Julia set, morphing between them
    vec2 z = q*1.45, c = u_p2.zw; float it = 0., trap = 1e9;
    for (int i = 0; i < 48; i++) { z = vec2(z.x*z.x - z.y*z.y, 2.*z.x*z.y) + c; trap = min(trap, abs(z.y) + .5*abs(z.x - .3)); if (dot(z, z) > 16.) break; it += 1.; }
    float sm = it/48.;
    col = it >= 48. ? u_ground*.5 + u_cool*.3*exp(-trap*5.) : mixOk(mixOk(u_ground, u_cool, sqrt(sm)*1.4), u_hot, smoothstep(.35, .9, sm)) + u_ink*exp(-trap*14.)*.5;
  }
  col = mix(col, u_ink, aa(abs(sdBox(f - pc, vec2(100.))) - 1.));
  vec3 U = ui(f);
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  float j = 2. + 1.5*sin(u_t*.7);                                            // the clock, printed three times out of register
  col = mix(col, u_hot, smoothstep(.3, .6, ui(f + vec2(j, 0.)).g)*.85);
  col = mix(col, u_cool, smoothstep(.3, .6, ui(f - vec2(j*.6, j*.6)).g)*.85);
  col = mix(col, u_ink, smoothstep(.3, .6, U.g));
  float sl = u_p3.x;                                                        // time out of joint: the screen is a slip of paper with its name on it
  if (sl > .002) {
    float th = fbm(f*.03 + 4.), fr = sl*1.3 - .15;
    vec2 sp = rot2(.05)*(f - vec2(256., 190.));
    float slip = sdBox(sp, vec2(190., 64.));
    vec3 sc = mix(u_ground*.5, paperOf(f), aa(slip));
    sc = mix(sc, u_ground*.85, smoothstep(.3, .6, U.b)*aa(slip));
    col = mix(col, sc, smoothstep(th - .04, th + .04, fr));
    col += u_hot*(1. - smoothstep(0., .035, abs(th - fr)))*step(.02, sl)*step(sl, .98);
  }
  o = vec4(col, 1.);
}`,
    bot: `
float ia(float d){ return clamp(.5 - d*42.*iResolution.z, 0., 1.); }   // coverage in icon units (42 px)
vec4 icon(int i, vec2 p, float t){
  vec3 c = u_ink; float a = 0.;
  if (i == 0) {                                                             // Ubik: the can and its mist
    float body = sdBox(p - vec2(-.2, -.2), vec2(.3, .55)) - .06, cap = sdBox(p - vec2(-.2, .5), vec2(.2, .12)), noz = sdBox(p - vec2(.02, .62), vec2(.1, .045));
    float d = min(body, min(cap, noz)); a = ia(d);
    c = mix(u_hot, u_ink, step(abs(p.y + .2), .12)*step(body, 0.)); c = mix(c, u_ground, step(abs(p.y + .2), .03)*step(body, 0.));
    float mist = 0.; for (int k = 0; k < 10; k++) { float fk = float(k); vec2 m = vec2(.18 + .07*fk, .62 + (.04 + .03*fk)*sin(fk*2.3 + t*4.)); mist += exp(-dot(p - m, p - m)*(260. - 12.*fk)); }
    c = mix(c, mixOk(u_ink, u_hot, .3), clamp(mist, 0., 1.)*(1. - a)); a = max(a, clamp(mist, 0., 1.));
  } else if (i == 1) {                                                      // an electric sheep
    float d = 1e9; for (int k = 0; k < 7; k++) { float fk = float(k)*.9; d = smin(d, length(p - vec2(-.1 + cos(fk)*.36, .05 + sin(fk)*.2)) - .24, .06); }
    float head = length((p - vec2(.55, .24))*vec2(1., 1.25)) - .17, legs = min(sdBox(p - vec2(-.32, -.42), vec2(.05, .17)), sdBox(p - vec2(.18, -.42), vec2(.05, .17)));
    a = ia(min(d, min(head, legs)));
    c = d < .0 ? u_ink*(.85 + .15*sin(length(p)*40.)) : u_ground*1.8 + u_cool*.2;
    float bolt = sdSeg(p, vec2(-.05, .1), vec2(.05, -.05)); c = mix(c, u_hot, ia(bolt - .04)*step(d, 0.));
  } else if (i == 2) {                                                      // the empathy box
    float box = sdBox(p - vec2(0., -.1), vec2(.5, .42)) - .04, scr = sdBox(p - vec2(0., -.05), vec2(.36, .27));
    float hl = sdBox(vec2(abs(p.x) - .62, p.y + .05), vec2(.07, .2));
    a = ia(min(box, hl)); c = mix(mixOk(u_cool, u_ground, .3), u_ink, step(0., -hl)*step(0., box));
    float glow = .5 + .5*sin(t*2.);
    c = mix(c, mixOk(u_hot, u_ink, .3 + .3*glow), ia(scr));
  } else if (i == 3) {                                                      // a scramble suit: one head, many people
    float hd = length(p*vec2(1., .82)) - .62; a = ia(hd);
    vec2 g = p*5. + vec2(0., floor(t*3.)*.0); vec2 id = floor(g); float best = 9.; vec2 bid = id;
    for (int y = -1; y <= 1; y++) for (int x = -1; x <= 1; x++) { vec2 n = id + vec2(float(x), float(y)); vec2 cp = n + .5 + .4*sin(hash22(n)*6.28 + t); float dd = length(g - cp); if (dd < best) { best = dd; bid = n; } }
    float hs = hash12(bid + floor(t*2.));
    c = mixOk(mixOk(u_ink, u_hot, hs), u_cool, step(.7, hs));
  } else if (i == 4) {                                                      // the fish, and the pink beam
    float d1 = abs(length(p - vec2(0., -.5)) - .78), d2 = abs(length(p - vec2(0., .5)) - .78);
    float fish = max(min(d1, d2) - .045, max(-p.x - .62, p.x - .78));
    float beam = sdSeg(p, vec2(.95, .95), vec2(-.1, -.05)) - .04*(1. + .3*sin(t*9.));
    a = max(ia(fish), ia(beam)*.9);
    c = ia(beam) > .5 ? mixOk(u_hot, u_ink, .25) : u_ink;
  } else {                                                                  // a little console, in a little room, holding a little console
    float room = abs(sdBox(p, vec2(.8, .75))) - .03;
    vec2 q = p - vec2(0., -.1); float dev = sdBox(q, vec2(.32, .52)) - .04;
    float s1 = sdBox(q - vec2(0., .24), vec2(.25, .19)), s2 = sdBox(q + vec2(0., .24), vec2(.25, .19));
    a = max(ia(room), ia(dev));
    c = mix(u_ink, mixOk(u_ground, u_ink, .3), ia(dev));
    vec2 r = q*3.2; float in2 = min(sdBox(r - vec2(0., .24), vec2(.25, .19)), sdBox(r + vec2(0., .24), vec2(.25, .19)));
    c = mix(c, mix(u_cool, mixOk(u_ground, u_ink, .3), ia(sdBox(r, vec2(.32, .52)))*(1. - ia(in2))), ia(min(s1, s2)));
  }
  return vec4(c, a);
}
void mainImage(out vec4 o, in vec2 f){
  vec2 cur = u_p0.xy, d = f - cur; float lens = u_p0.z, r2 = dot(d, d);
  vec2 g = f - d*.24*lens*exp(-r2/9000.);                                    // the cursor is a gravity well: things near it swell toward it
  vec3 col = u_ground + u_cool*.05*(.5 + .5*sin(sqrt(r2)*.12 - u_t*3.))*exp(-r2/60000.);
  vec3 U = ui(g);
  float cx = floor((g.x - 24.)/158.), ry = floor(((384. - g.y) - 50.)/156.);
  if (cx >= 0. && cx < 3. && ry >= 0. && ry < 2.) {
    int ti = int(ry)*3 + int(cx); vec2 tc = tileC(ti), lp = g - tc;
    float pres = ti < 4 ? u_p1[ti] : u_p2[ti - 4];
    vec3 tcol = mix(col, mixOk(u_ground, u_cool, .22), smoothstep(.3, .6, U.g));
    vec4 ic = icon(ti, (lp - vec2(0., 20.))/42., u_t + float(ti));
    tcol = mix(tcol, ic.rgb, ic.a*smoothstep(.3, .6, U.g));
    tcol = mix(tcol, u_ink, smoothstep(.3, .6, U.r));
    vec2 sp = rot2(ANG[ti])*lp;                                              // the slip of paper it really is
    float slip = sdBox(sp - vec2(0., 7.), vec2(66., 24.));
    vec3 scol = col*(1. - .5*exp(-max(sdBox(sp - vec2(4., 2.), vec2(66., 24.)), 0.)/5.));
    scol = mix(scol, paperOf(g), aa(slip));
    scol = mix(scol, u_ground*.85, smoothstep(.3, .6, U.b)*aa(slip));
    float th = fbm(g*.035 + float(ti)*3.7), fr = 1.15*(1. - pres) - .08;
    col = mix(tcol, scol, smoothstep(th - .04, th + .04, fr));
    col += u_hot*(1. - smoothstep(0., .03, abs(th - fr)))*step(.02, 1. - pres)*step(.02, pres)*.9;
  } else col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  vec2 z = d/70.; float trap = 1e9; vec2 c = vec2(-.745, .113) + .03*vec2(cos(u_t*.7), sin(u_t*.9));
  for (int i = 0; i < 14; i++) { z = vec2(z.x*z.x - z.y*z.y, 2.*z.x*z.y) + c; trap = min(trap, abs(length(z) - .6)); if (dot(z, z) > 8.) break; }
  col += u_hot*exp(-trap*18.)*lens*.5*exp(-r2/14000.);                        // the event horizon
  col = mix(col, u_hot, (1. - smoothstep(0., 2.5, abs(sdBox(f - tileC(int(u_sel + .5)), vec2(77., 76.)) - 1.)))*lens);
  o = vec4(col, 1.);
}`
  }
});

/* ═══════════════════════ PAUSE · Ubik's cold-pac ═══════════════════════ */
const PAUSE_OPTS = ['RESUME', 'BACK TO CARTRIDGES', 'REBOOT REALITY'];
scene({
  id: 'pause', name: 'PAUSED', status: 'COLD-PAC', element: 'Pause screen', book: 'Ubik',
  pal: { ground: '#0A1018', ink: '#E2F4FF', hot: '#9FE3FF', cool: '#4A78A8' },
  hint: ['The game is frozen, and frost grows over it from the edges.', 'Stay too long and something writes to you through the ice.', 'START or B resumes. Up and down choose, A confirms.'],
  enter(S, G, o) { S.from = o.from; S.sel = 0; },
  update(S, G, I) {
    if (I.press('UP')) S.sel = (S.sel + 2) % 3; if (I.press('DOWN')) S.sel = (S.sel + 1) % 3;
    let act = I.press('A') ? S.sel : -1;
    if (I.tap) for (let i = 0; i < 3; i++) { const y = 110 + i * 70; if (I.tap[1] >= y && I.tap[1] <= y + 54 && I.tap[0] >= 56 && I.tap[0] <= 456) act = i; }
    if (I.press('START') || I.press('B')) act = 0;
    if (act === 0 && S.from) G.go(S.from.sc.id, 'cut', { resume: S.from });
    else if (act === 1 || (act === 0 && !S.from)) G.go('home', 'regress');
    else if (act === 2) G.go('boot', 'tape');
  },
  sig: (S, G, s) => s ? `${S.sel}` : `${Math.floor(S.t)}`,
  ui: {
    top(U, S) {
      U.text('PAUSED', 256, 168, { f: 'jac', size: 92, align: 'center' });
      const s = Math.floor(S.t); U.text(`COLD-PAC  00:${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`, 256, 206, { size: 16, align: 'center', ch: 'g' });
      U.wrap('Do not stay paused too long. Something in here is eating the other saves.', 76, 268, 360, { f: 'cou', w: 700, size: 20, lh: 24, ch: 'b' });
    },
    bot(U, S) {
      U.text('START RESUMES', 256, 62, { size: 8, align: 'center', a: .7, track: 2 });
      PAUSE_OPTS.forEach((t, i) => { const y = 110 + i * 70; U.rect(56, y, 400, 54, { ch: i === S.sel ? 'gb' : 'g', r: 4 }); U.text(t, 256, y + 34, { size: 16, align: 'center' }); });
    }
  },
  tex(h) { const a = h.targets.get('frz0'), b = h.targets.get('frz1'); return { u_aux: a ? a.tex : h.H.white, u_aux2: b ? b.tex : h.H.white }; },
  common: `
vec3 frozen(sampler2D t, vec2 f){ vec2 e = vec2(1.5, 0.); vec3 c = (texture(t, (f + e)/LS).rgb + texture(t, (f - e)/LS).rgb + texture(t, (f + e.yx)/LS).rgb + texture(t, (f - e.yx)/LS).rgb)*.25;
  return mix(c, vec3(luma(c)), .55)*.72; }
vec3 frost(vec2 f, vec3 base, float t){
  float de = min(min(f.x, 512. - f.x), min(f.y, 384. - f.y));
  float front = t*15. + (fbm(f*.025 + 2.) - .5)*90. + 4.;
  float m = smoothstep(front, front - 12., de);
  float cr = 0.;
  for (int i = 0; i < 3; i++) { vec2 q = rot2(float(i)*1.0472 + .3)*f; float r = 1. - abs(vnoise(q*vec2(.11, .014) + float(i)*7.)*2. - 1.); cr = max(cr, pow(r, 9.)); }
  float fine = 1. - abs(vnoise(f*.4)*2. - 1.);
  vec3 ice = mix(base*vec3(.55, .72, .95) + u_cool*.25, u_ink, clamp(cr*.85 + pow(fine, 14.)*.35, 0., 1.));
  return mix(base, ice, m) + u_hot*.5*(1. - smoothstep(0., 3., abs(de - front)))*step(de, front + 4.);
}`,
  glsl: {
    top: `
void mainImage(out vec4 o, in vec2 f){
  vec3 col = frost(f, frozen(u_aux, f), u_t);
  vec3 U = ui(f);
  col = mix(col, col*.35, smoothstep(.05, .5, uiL(f, 3.).r));
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  col = mix(col, u_hot, smoothstep(.3, .6, U.g));
  float msg = smoothstep(.3, .6, ui(f + vec2(sin(f.y*.2 + u_t*3.), 0.)).b)*smoothstep(6., 9., u_t);   // it writes through the ice
  col = mix(col, mixOk(u_hot, u_ink, .5), msg);
  o = vec4(col, 1.);
}`,
    bot: `
void mainImage(out vec4 o, in vec2 f){
  vec3 col = frost(f, frozen(u_aux2, f), u_t);
  vec3 U = ui(f);
  col = mix(col, mix(u_ground*.8, u_cool*.6, smoothstep(.3, .6, U.b)), smoothstep(.3, .6, U.g)*.85);
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  o = vec4(col, 1.);
}`
  }
});

/* ═══════════════════════ GAME OVER · kipple garden ═══════════════════════ */
scene({
  id: 'over', name: 'GAME OVER', status: 'GAME OVER', element: 'Game over and the continue countdown', book: 'Do Androids Dream of Electric Sheep?',
  pal: { ground: '#0A0F0D', ink: '#F4FFE9', hot: '#FF4F79', cool: '#8BFFB0' },
  drain: false,
  hint: ['The words GAME OVER are planted and grow a coral across both screens and the hinge.', 'Each number of the countdown is planted too. Touch to plant more.', 'A or a touch continues. At zero the console reboots.'],
  enter(S, G, o) { S.reason = o.reason || 'GAME OVER'; S.digit = 9; S.seed = 1; S.seedB = 1; },
  update(S, G, I, dt) {
    const d = Math.max(0, 9 - Math.floor(S.t));
    if (d !== S.digit) { S.digit = d; S.seedB = 1; }
    S.seed = Math.max(0, S.seed - dt * 1.2); S.seedB = Math.max(0, S.seedB - dt * 4);
    if (S.t > 10) { G.reality = 1; G.go('boot', 'tape'); return; }
    if ((I.tap && S.t > 1.5) || I.press('A')) { G.reality = Math.max(G.reality, .6); G.go('home', 'scramble'); }
    if (I.press('START')) G.go('boot', 'tape');
  },
  sig: (S, G, s) => s ? `${S.digit}` : S.reason,
  ui: {
    top(U, S) { U.text('GAME', 256, 150, { f: 'jac', size: 120, align: 'center' }); U.text('OVER', 256, 262, { f: 'jac', size: 120, align: 'center' });
      U.text(S.reason, 256, 330, { size: 16, align: 'center', ch: 'g', track: 1 }); },
    bot(U, S) { U.text('CONTINUE?', 256, 56, { size: 16, align: 'center', ch: 'g' }); U.text(String(S.digit), 256, 290, { size: 210, w: 700, align: 'center' });
      U.text('TOUCH OR PRESS A', 256, 360, { size: 16, align: 'center', ch: 'g' }); }
  },
  tex(h, S, G, inst) {
    const [a, b] = inst.L;
    if (!S.init) { const R = h.sims.get('over.rd'); if (R) R.steps = 0; S.init = 1; }
    const rd = h.sim('over.rd', 'over.sim', 128, 215, { steps: 4, float: true, linear: true, textures: { u_ui: a.tex || h.H.white, u_aux: b.tex || h.H.white }, uniforms: { u_p0: [S.seed, S.seedB, 0, 0] } });
    return { u_mem: rd };
  },
  glsl: {
    sim: `
void mainImage(out vec4 o, in vec2 f){
  vec2 C = f*4.;
  vec4 c = prev(f);
  if (iFrame == 0) c = vec4(1., 0., 0., 0.);
  vec4 n = prev(f + vec2(0., 1.)) + prev(f - vec2(0., 1.)) + prev(f + vec2(1., 0.)) + prev(f - vec2(1., 0.));
  vec4 g = prev(f + vec2(1.)) + prev(f - vec2(1.)) + prev(f + vec2(1., -1.)) + prev(f + vec2(-1., 1.));
  vec2 lap = iFrame == 0 ? vec2(0.) : -c.xy + .2*n.xy + .05*g.xy;
  vec2 q = C/CS;
  float m = .5 + .5*sin(q.x*5. + q.y*7. + vnoise(q*3.)*4. + iTime*.05);   // feed and kill drift: mazes here, coral there
  float F = mix(.029, .055, m), k = mix(.057, .062, m);
  float A = c.x, B = c.y, r = A*B*B;
  A += lap.x - r + F*(1. - A);
  B += .5*lap.y + r - (k + F)*B;
  if (C.y > LS.y + DS_GAP) B = max(B, smoothstep(.4, .7, textureLod(u_ui, (C - vec2(0., LS.y + DS_GAP))/LS, 1.).r)*u_p0.x);   // only the strokes are planted
  if (C.y < LS.y) B = max(B, smoothstep(.4, .7, textureLod(u_aux, C/LS, 1.).r)*u_p0.y);
  vec2 d = C - iTouch.xy; B = max(B, iTouch.z*step(dot(d, d), 160.));
  B *= 1. - .08*smoothstep(.8, .86, vnoise(C*.006 + vec2(iTime*.031, -iTime*.017)));
  o = vec4(clamp(A, 0., 1.), clamp(B, 0., 1.), c.z*.985 + B*.015, 1.);
}`,
    top: `
vec3 bloom(vec2 C){
  vec2 uv = C/CS, px = 1./vec2(textureSize(u_mem, 0));
  vec4 s = texture(u_mem, uv);
  float bx = texture(u_mem, uv + vec2(px.x, 0.)).y - texture(u_mem, uv - vec2(px.x, 0.)).y, by = texture(u_mem, uv + vec2(0., px.y)).y - texture(u_mem, uv - vec2(0., px.y)).y;
  vec3 n = normalize(vec3(-bx, -by, .3)), l = normalize(vec3(-.5, .6, .65));
  float t = smoothstep(.06, .32, s.y), age = smoothstep(.05, .2, s.z);
  vec3 tint = mixOk(mixOk(u_hot, u_ink, t*.5), mixOk(u_cool*.45, u_cool, t), age);
  float dif = max(dot(n, l), 0.), sp = pow(max(dot(reflect(-l, n), vec3(0., 0., 1.)), 0.), 18.);
  return mix(u_ground + u_cool*.04*s.z, tint*(.35 + .85*dif) + sp*.5, t);
}
void mainImage(out vec4 o, in vec2 f){
  vec3 col = bloom(dsCanvas(f)); vec3 U = ui(f);
  col += u_hot*uiL(f, 3.).r*.18; col = mix(col, u_ink, smoothstep(.3, .6, U.r)*.22);
  col = mix(col, u_ink, smoothstep(.3, .6, U.g));
  o = vec4(col, 1.);
}`,
    bot: `
vec3 bloom(vec2 C){
  vec2 uv = C/CS, px = 1./vec2(textureSize(u_mem, 0));
  vec4 s = texture(u_mem, uv);
  float bx = texture(u_mem, uv + vec2(px.x, 0.)).y - texture(u_mem, uv - vec2(px.x, 0.)).y, by = texture(u_mem, uv + vec2(0., px.y)).y - texture(u_mem, uv - vec2(0., px.y)).y;
  vec3 n = normalize(vec3(-bx, -by, .3)), l = normalize(vec3(-.5, .6, .65));
  float t = smoothstep(.06, .32, s.y), age = smoothstep(.05, .2, s.z);
  vec3 tint = mixOk(mixOk(u_hot, u_ink, t*.5), mixOk(u_cool*.45, u_cool, t), age);
  float dif = max(dot(n, l), 0.), sp = pow(max(dot(reflect(-l, n), vec3(0., 0., 1.)), 0.), 18.);
  return mix(u_ground + u_cool*.04*s.z, tint*(.35 + .85*dif) + sp*.5, t);
}
void mainImage(out vec4 o, in vec2 f){
  vec3 col = bloom(f); vec3 U = ui(f);
  col = mix(col, u_ink, smoothstep(.3, .6, U.r)*.3);
  col = mix(col, u_ink, smoothstep(.3, .6, U.g)*(.55 + .45*step(.5, fract(u_t))));
  vec2 d = f - iTouch.xy; col = mix(col, u_ink, (1. - smoothstep(1., 2.5, abs(length(d) - 13.)))*iTouch.w);
  o = vec4(col, 1.);
}`
  }
});
})();
