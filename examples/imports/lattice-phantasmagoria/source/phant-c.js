/* phant-c.js — cartridges 4–6: a dialogue box with a scramble suit, a world map under the pink beam, a layout with the console inside it. */
(function () {
'use strict';
const PH = window.PHANT = window.PHANT || { list: [], byId: {} };
const scene = (s) => { PH.list.push(s); PH.byId[s.id] = s; return s; };
const LH = 384;

/* ═══════════════════════ SCRAMBLE SUIT · the dialogue box (A Scanner Darkly) ═══════════════════════ */
const LINES = [
  'Hello. I am wearing the {suit|scramble suit|blur}, so you see {everyone|a million people|nobody} and {no one|someone|you}.',
  'Somebody in this {house|room|console} is an {informant|addict|agent}. Maybe {the console|me|you}.',
  'The {left|right} half of your brain is reading this. The {right|left} half {already knows|disagrees|is somebody else}.',
  'Are you ready to turn yourself in?',
  'Either way I write the report. I am the one who writes the reports on {myself|you|Bob|Fred}.'
];
const NAMES = ['FRED', 'BOB', 'ARCTOR', 'FRED', '???', 'YOU'];
const plain = s => s.replace(/\{([^}|]*)[^}]*\}/g, '$1');
const roll = (s, r) => s.replace(/\{([^}]*)\}/g, (_, a) => { const o = a.split('|'); return o[Math.floor(r() * o.length)]; });
const SUIT = `
vec4 hash4(vec2 p){ return vec4(hash22(p), hash22(p + 17.31)); }
// one face, from eight numbers
vec3 face(vec2 p, vec4 a, vec4 b, float talk){
  vec3 skin = mixOk(mixOk(u_ink, u_hot, .2 + .35*a.x), u_ground, .12 + .35*a.y*a.y);
  vec3 hair = mixOk(u_ground, mixOk(u_hot, u_cool, a.z), .2 + .5*b.w*b.w);
  vec3 col = mixOk(u_ground, u_cool, .2 + .25*b.x);
  float sh = length(max(abs(p - vec2(0., -2.3)) - vec2(.85 + .3*a.x, 1.15), 0.)) - .5;
  if (sh < 0.) { col = mixOk(mixOk(u_cool, u_hot, b.y), u_ground, .3 + .3*a.w);
    float pat = a.z < .33 ? step(.5, fract(p.x*4.*(1. + b.z))) : a.z < .66 ? step(.5, fract((p.x + p.y)*3.)) : step(.25, fract(p.x*2.)*fract(p.y*2.));
    col = mix(col, col*.68, pat*.6); }
  if (sdBox(p - vec2(0., -.95), vec2(.2 + .05*b.x, .3)) < 0.) col = skin*.78;
  float hb = length((p - vec2(0., .1))*vec2(1., .9)) - (.86 + .2*a.w);
  if (hb < 0. && p.y > -.45 + .3*a.z) col = hair;
  float head = length(p*vec2(1., .8 + .15*a.z)) - (.76 + .08*b.y);
  if (head < 0.) {
    col = skin*(.84 + .22*p.x*(b.x - .5)*2.);
    if (p.y > .36 - .25*b.x + .08*sin(p.x*(4. + 6.*a.w))) col = hair;
    vec2 e = vec2(abs(p.x) - (.27 + .1*b.y), p.y - .07 - .06*a.w);
    if (length(e*vec2(1., 1.6 + .8*b.z)) < .1 + .04*b.w) { col = u_ink; if (length(e - vec2(.025*sin(u_t*.7), 0.)) < .045 + .02*a.x) col = u_ground*.4; }
    if (sdSeg(vec2(abs(p.x), p.y), vec2(.13, .25 + .05*a.y), vec2(.45, .25 - .08*(b.z - .5))) < .028) col = hair*.7;
    if (sdSeg(p, vec2(0., .01), vec2(.06*(a.w - .5), -.2 - .1*a.x)) < .03) col = skin*.62;
    if (length((p - vec2(0., -.44 - .04*b.w))/vec2(.16 + .1*b.x, .025 + .1*talk)) < 1.) col = mixOk(u_hot, u_ground, .55);
  }
  return col;
}
// the suit: a mosaic of fragments, each cut from somebody else, drifting and re-casting; the two hemispheres wear different people
vec3 suit(vec2 C, float talk){
  vec2 g = C/22., id = floor(g); float best = 9., b2 = 9.; vec2 bid = id;
  for (int y = -1; y <= 1; y++) for (int x = -1; x <= 1; x++) { vec2 n = id + vec2(float(x), float(y)); vec2 cp = n + .5 + .38*sin(hash22(n)*6.28 + u_t*(.5 + hash12(n)));
    float d = length(g - cp); if (d < best) { b2 = best; best = d; bid = n; } else b2 = min(b2, d); }
  float ph = u_t*(.35 + .7*hash12(bid + 5.)) + hash12(bid)*10., side = step(256., C.x)*50.;
  float k = smoothstep(.75, 1., fract(ph));
  vec4 A = mix(hash4(bid + floor(ph) + side), hash4(bid + floor(ph) + 1. + side), k), B = mix(hash4(bid*1.7 + floor(ph) + 9. + side), hash4(bid*1.7 + floor(ph) + 10. + side), k);
  vec2 off = (hash22(bid + 3.) - .5)*12.;
  vec3 col = face((C + off - vec2(256., 384. + DS_GAP + 178.))/128., A, B, talk);
  float edge = b2 - best;
  return col*(.82 + .18*smoothstep(0., .12, edge)) + u_hot*.12*(1. - smoothstep(0., .05, edge));
}`;
scene({
  id: 'scramble', name: 'SCRAMBLE SUIT', status: 'DIALOGUE', element: 'Dialogue box and speaker portrait', book: 'A Scanner Darkly', cart: true,
  pal: { ground: '#0E0F13', ink: '#E7E9EC', hot: '#FF3FC9', cool: '#35EFC9' },
  hint: ['The speaker wears a scramble suit: a mosaic of a million other people, across both screens.', 'A skips the typing, and the letters smear. The other half of your brain reads a different version above the box.', 'At the question, A answers yes and B answers no. It makes no difference.'],
  enter(S) { S.line = 0; S.chars = 0; S.smear = 0; S.alt = LINES[0]; S.rollT = 0; S.name = 0; },
  update(S, G, I, dt) {
    const txt = plain(LINES[S.line]); const done = S.chars >= txt.length;
    if (!done) S.chars = Math.min(txt.length, S.chars + dt * 30);
    S.smear = Math.max(0, S.smear - dt * 1.4);
    S.rollT -= dt; if (S.rollT <= 0) { S.alt = roll(LINES[S.line], Math.random); S.rollT = .7; S.name = (S.name + 1) % NAMES.length; }
    const ask = S.line === 3 && done;
    if (I.press('A') || (I.tap && !ask)) {
      if (!done) { S.chars = txt.length; S.smear = 1; }
      else { S.line = S.line === 3 ? 4 : (S.line + 1) % LINES.length; S.chars = 0; S.rollT = 0; S.answer = ask ? 'YES' : S.answer; }
    } else if (I.press('B')) { if (ask) { S.line = 4; S.chars = 0; S.answer = 'NO'; } else G.go('home', 'regress'); }
    S.talk = done ? 0 : .5 + .5 * Math.sin(S.t * 22);
  },
  focus: () => [256, 120],
  sig: (S, G, s) => s ? `${S.line}|${Math.floor(S.chars)}|${S.alt}` : NAMES[S.name],
  ui: {
    top(U, S) { U.rect(16, 40, 176, 26, { ch: 'g', r: 3 }); U.text(`SPEAKER  ${NAMES[S.name]}`, 26, 59, { size: 16 }); },
    bot(U, S) {
      const txt = plain(LINES[S.line]).slice(0, Math.floor(S.chars));
      U.wrap(S.alt.replace(/[{}]/g, ''), 30, 40, 452, { f: 'cou', w: 700, size: 19, lh: 22, ch: 'b' });
      U.rect(12, 156, 488, 216, { ch: 'g', r: 6 }); U.frame(21, 165, 470, 198, { ch: 'r', lw: 1.5, r: 3 });
      U.rect(28, 140, 132, 28, { ch: 'g', r: 4 }); U.text(NAMES[S.name], 94, 160, { size: 16, align: 'center' });
      U.wrap(txt, 38, 204, 436, { f: 'vt', size: 26, lh: 26 });
      if (S.line === 3 && S.chars >= plain(LINES[3]).length) { U.text('A  YES', 120, 340, { size: 16, align: 'center', ch: 'rb' }); U.text('B  NO', 392, 340, { size: 16, align: 'center', ch: 'rb' }); }
    }
  },
  uni: (S) => ({ u_p0: [S.smear, S.chars >= plain(LINES[S.line]).length && S.line !== 3 ? 1 : 0, S.talk || 0, 0] }),
  common: SUIT,
  glsl: {
    top: `
void mainImage(out vec4 o, in vec2 f){
  vec3 col = suit(dsCanvas(f), u_p0.z);
  col *= .92 + .08*sin(f.y*1.6 + u_t*8.);
  vec3 U = ui(f);
  col = mix(col, u_ground*.9, smoothstep(.3, .6, U.g));
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  o = vec4(col, 1.);
}`,
    bot: `
void mainImage(out vec4 o, in vec2 f){
  vec3 col = suit(f, u_p0.z), U = ui(f);
  col = mix(col, col*.3, smoothstep(.05, .5, uiL(f, 2.).b));
  col = mix(col, mixOk(u_cool, u_ink, .35), smoothstep(.3, .6, U.b)*(.75 + .25*step(.5, hash12(vec2(floor(u_t*8.), floor(f.y/11.))))));
  col = mix(col, u_ground*.94, smoothstep(.3, .6, U.g));
  float sm = u_p0.x, trail = 0.;                                             // skipped text smears
  for (int i = 1; i < 8; i++) trail = max(trail, smoothstep(.3, .6, ui(f + vec2(float(i)*sm*9., 0.)).r)*(1. - float(i)/8.));
  col = mix(col, u_hot, trail*sm*step(f.y, 230.));
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  vec2 q = f - vec2(470., 30. + 3.*sin(u_t*4.));
  col = mix(col, u_hot, aa(max(q.y - 4., -6. + abs(q.x)*1.2 - q.y))*u_p0.y*step(.4, fract(u_t*1.2)));
  o = vec4(col, 1.);
}`
  }
});

/* ═══════════════════════ PINK BEAM · the world map (VALIS) ═══════════════════════ */
const MSGS = ['THE EMPIRE NEVER ENDED', 'TIME IS NOT REAL', 'YOU ARE ALSO THIS ONE', 'THE PRISON IS MADE OF IRON AND IT IS EVERYWHERE', 'IT IS 70 AD AND IT IS 1974', 'A FISH WAS DRAWN ON THE DOOR'];
scene({
  id: 'beam', name: 'PINK BEAM', status: 'WORLD MAP', element: 'World map, minimap and the two screens', book: 'VALIS', cart: true,
  pal: { ground: '#09070D', ink: '#FAEADF', hot: '#FF4FA3', cool: '#4D63E8' },
  hint: ['The bottom screen is the map in 1974. The top screen is the same ground in 70 AD, as a mosaic.', 'An iron prison closes in from the edges. Press A to fire the pink beam from the fish, through the hinge, at your marker.', 'Move with the arrows or touch the map.'],
  enter(S) { S.pw = [256, 192]; S.cam = [0, 0]; S.prison = PH.prison ?? .3; S.beamT = 9; S.msg = -1; S.target = null; },
  update(S, G, I, dt) {
    const v = 110 * dt;
    if (I.held('LEFT')) S.pw[0] -= v; if (I.held('RIGHT')) S.pw[0] += v; if (I.held('UP')) S.pw[1] += v; if (I.held('DOWN')) S.pw[1] -= v;
    if (I.touch.down) S.target = [I.touch.x + S.cam[0], I.touch.y + S.cam[1]];
    if (S.target) { const dx = S.target[0] - S.pw[0], dy = S.target[1] - S.pw[1], d = Math.hypot(dx, dy); if (d < 2) S.target = null; else { S.pw[0] += dx / d * Math.min(d, v * 1.6); S.pw[1] += dy / d * Math.min(d, v * 1.6); } }
    const k = 1 - Math.exp(-dt * 2.5); S.cam[0] += (S.pw[0] - 256 - S.cam[0]) * k; S.cam[1] += (S.pw[1] - 192 - S.cam[1]) * k;
    S.beamT += dt;
    if (I.press('A') && S.beamT > .9) { S.beamT = 0; S.prison = Math.max(0, S.prison - .2); S.msg = (S.msg + 1) % MSGS.length; G.splat(S.pw[0] - S.cam[0], S.pw[1] - S.cam[1], .45); }
    S.prison = Math.min(1, S.prison + dt * .011); PH.prison = S.prison;
    if (S.prison >= 1) { PH.prison = .3; G.over('THE EMPIRE NEVER ENDED'); }
    if (I.press('B')) G.go('home', 'regress');
  },
  focus: (S) => [S.pw[0] - S.cam[0], S.pw[1] - S.cam[1]],
  sig: (S, G, s) => s ? `${S.msg}|${Math.round(S.pw[0] / 8)}|${Math.round(S.pw[1] / 8)}` : `${S.msg}`,
  ui: {
    top(U, S) { U.text('AD 70', 24, 80, { f: 'jac', size: 44 }); U.text('1974', 488, 80, { f: 'jac', size: 44, align: 'right' });
      U.text(S.msg >= 0 ? MSGS[S.msg] : 'THE EMPIRE NEVER ENDED', 256, 352, { f: 'jac', size: S.msg === 3 ? 22 : 30, align: 'center', ch: 'b' }); },
    bot(U, S) {
      const lat = (33.7 + S.pw[1] / 4000).toFixed(2), lon = (117.9 - S.pw[0] / 4000).toFixed(2);
      U.text('WORLD MAP', 16, 26, { size: 16 }); U.text(`${lat}N ${lon}W`, 496, 26, { size: 8, align: 'right', track: 1 });
      U.text(S.msg >= 0 ? `RECEIVED: ${MSGS[S.msg]}` : 'A  FIRE THE BEAM', 16, 372, { f: 'vt', size: 22, ch: S.msg >= 0 ? 'b' : 'r' });
    }
  },
  uni: (S) => ({ u_p0: [S.cam[0], S.cam[1], S.pw[0] - S.cam[0], S.pw[1] - S.cam[1]], u_p1: [S.prison, S.beamT, 0, 0] }),
  common: `
float land(vec2 w){ return fbm(w*.0045 + vec2(3.1, 1.7))*1.1 + .35*fbm(w*.018 + 9.) - .78; }
vec2 FISH(){ return vec2(256., 384. + DS_GAP + 196.); }
// the ichthys: the top of the lower circle and the bottom of the upper one meet at the head and cross into the tail
float fish(vec2 p){ float d1 = p.y > -.5 ? abs(length(p - vec2(0., -.5)) - .78) : 9., d2 = p.y < .5 ? abs(length(p - vec2(0., .5)) - .78) : 9.;
  return max(min(d1, d2) - .05, max(-.6 - p.x, p.x - .78)); }
vec3 beam(vec2 C, vec3 col){
  float bt = u_p1.y, I = smoothstep(1.3, 0., bt)*step(0., bt);
  if (I <= 0.) return col;
  vec2 A = FISH(), B = u_p0.zw; vec2 dir = normalize(B - A);
  float d = sdSeg(C, A, B), along = dot(C - A, dir);
  float pulse = step(.6, fract(along/16. - bt*9.))*exp(-d*d/6.);
  return col + u_hot*(exp(-d*d/16.)*1.6 + exp(-d/20.)*.45 + pulse*1.2)*I + u_ink*exp(-d*d/2.)*I;
}`,
  glsl: {
    top: `
void mainImage(out vec4 o, in vec2 f){
  vec2 C = dsCanvas(f), w = f + u_p0.xy + vec2(0., 300.);
  float T = 7.; vec2 cell = floor(w/T), cq = fract(w/T) - .5 + (hash22(cell) - .5)*.18;
  float h = land((cell + .5)*T), hs = hash12(cell);
  vec3 tess = h < 0. ? mixOk(mixOk(u_cool, u_ground, .35), u_ink, .12*hs + .1*step(.8, fract(h*10.))) : mixOk(mixOk(u_ink, u_hot, .25 + .25*hs), u_ground, smoothstep(.0, .3, h)*.5);
  vec3 col = mix(tess*(.88 + .14*hs), u_ground*.5, smoothstep(.36, .46, max(abs(cq.x), abs(cq.y))));
  col = mix(col, u_ground*.25, (1. - smoothstep(1.5, 3., abs(mod(f.x, 34.) - 17.)))*u_p1.x*.45);   // the bars are already here, faintly
  vec2 fp = (f - vec2(256., 196.))/62.;
  float fd = fish(fp)*62.;
  col = mix(col, u_ground, aa(fd - 5.)*.85);
  col = mix(col, mixOk(u_hot, u_ink, .2 + .2*sin(u_t*3.)), aa(fd));
  col += u_hot*exp(-max(fd, 0.)/14.)*.35*(.6 + .4*sin(u_t*2.));
  col = beam(C, col);
  vec3 U = ui(f);
  col = mix(col, col*.3, smoothstep(.05, .5, uiL(f, 2.).r));
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  col = mix(col, u_hot, smoothstep(.3, .6, U.b)*smoothstep(2.5, .2, u_p1.y));
  o = vec4(col, 1.);
}`,
    bot: `
float hypEdge(vec2 z, out float n){
  const float a = PI/7., cr = 1.7461, cd = 2.0122;                          // {7,3}: the circle orthogonal to the rim that meets the mirror at 60°
  vec2 nl = vec2(-sin(a), cos(a)); n = 0.;
  for (int i = 0; i < 16; i++) {
    bool done = true;
    if (z.y < 0.) { z.y = -z.y; n += 1.; done = false; }
    float d = dot(z, nl); if (d > 0.) { z -= 2.*d*nl; n += 1.; done = false; }
    vec2 q = z - vec2(cd, 0.); float l2 = dot(q, q); if (l2 < cr*cr) { z = vec2(cd, 0.) + q*cr*cr/l2; n += 1.; done = false; }
    if (done) break;
  }
  return abs(length(z - vec2(cd, 0.)) - cr);
}
void mainImage(out vec4 o, in vec2 f){
  vec2 w = f + u_p0.xy;
  float h = land(w), hx = land(w + vec2(2., 0.)) - h, hy = land(w + vec2(0., 2.)) - h;
  vec3 col;
  if (h < 0.) col = mixOk(u_ground, u_cool, .3 + .25*smoothstep(-.25, 0., h)) + u_cool*.12*step(.85, fract(h*14. - u_t*.15))*smoothstep(-.3, 0., h);
  else { col = mixOk(mixOk(u_ground, u_cool, .5), u_ink, smoothstep(.0, .3, h)*.55)*(.72 + .5*clamp(.5 - (hx - hy)*30., 0., 1.)); col = mix(col, u_ink*.6, step(.9, fract(h*20.))*.25); }
  col = mix(col, u_ink, 1. - smoothstep(.5, 1.5, abs(h)/max(length(vec2(hx, hy))/2., 1e-4)));
  vec2 gq = abs(fract(w/64.) - .5)*64.; col = mix(col, u_ink, (1. - smoothstep(0., 1., min(gq.x, gq.y)))*.08);
  float R = mix(470., 140., u_p1.x);                                         // the Black Iron Prison, a hyperbolic tiling whose disk shrinks
  vec2 z = rot2(u_t*.03)*(f - vec2(256., 192.))/R; float r = length(z);
  vec3 iron = mixOk(u_ground, u_ink, .14)*(.75 + .25*vnoise(f*vec2(.02, .35))) + u_hot*.05;
  if (r >= 1.) col = mix(col, iron, .93);
  else { float n, e = hypEdge(z, n)*R, wd = 2.2*(1. - r*r) + .4;
    col = mix(col, col*.82, mod(n, 2.)*.5*u_p1.x);
    col = mix(col, iron, 1. - smoothstep(wd, wd + 1.4, e));
    col += u_hot*.25*exp(-e/2.)*smoothstep(.6, 1., r); }
  col = mix(col, iron, (1. - smoothstep(0., 3., abs(r - 1.)*R))*.9);
  vec2 P = u_p0.zw; float pd = length(f - P);
  col = mix(col, u_ground, aa(abs(pd - 7.) - 2.5)); col = mix(col, u_hot, aa(abs(pd - 7.) - 1.2)); col = mix(col, u_ink, aa(pd - 2.));
  float bt = u_p1.y; col += u_hot*exp(-abs(pd - bt*240.)/3.)*exp(-bt*2.)*step(bt, 2.);
  col = beam(f, col);
  vec3 U = ui(f);
  col = mix(col, col*.3, smoothstep(.05, .5, uiL(f, 2.).r + uiL(f, 2.).b));
  col = mix(col, u_ink, smoothstep(.3, .6, U.r)); col = mix(col, u_hot, smoothstep(.3, .6, U.b));
  o = vec4(col, 1.);
}`
  }
});

/* ═══════════════════════ LAYOUT · the console inside the console (The Three Stigmata of Palmer Eldritch) ═══════════════════════ */
scene({
  id: 'layout', name: 'LAYOUT', status: 'LAYOUT', element: 'The console itself', book: 'The Three Stigmata of Palmer Eldritch', cart: true,
  pal: { ground: '#1C1530', ink: '#FFF0D8', hot: '#FF7A59', cool: '#6BDDF2' },
  hint: ['A little room, and on its table a little console showing these two screens, which show the little console, and so on.', 'Hold A to chew Can-D and go into the layout. Each level down, more of the room has Palmer Eldritch’s marks.', 'The recursion is real: the little screens show the last frame of the big ones.'],
  enter(S) { S.u = 0; S.depth = 0; },
  update(S, G, I, dt) {
    if (I.held('A')) { S.u += dt * .42; if (S.u >= 1) { S.u -= 1; S.depth++; } }
    else S.u = Math.max(0, S.u - dt * .6);
    if (I.press('B')) G.go('home', 'regress');
  },
  focus: () => [256, 170],
  sig: (S, G, s) => s ? '' : `${S.depth}`,
  ui: {
    top(U, S) { U.text(`LAYOUT  DEPTH ${S.depth}`, 24, 60, { size: 16, ch: 'g' }); if (S.depth >= 3) U.text('PALMER ELDRITCH IS IN THE LAYOUT', 24, 84, { size: 8, ch: 'g', track: 1 }); },
    bot(U) { U.text('HOLD A TO CHEW CAN-D', 256, 372, { size: 16, align: 'center' }); }
  },
  tex(h) { const a = h.sims.get('comp0'), b = h.sims.get('comp1'); return { u_aux: a ? a.a.tex : h.H.white, u_aux2: b ? b.a.tex : h.H.white }; },
  uni: (S, G) => ({ u_p0: [S.u, S.depth, 0, 0], u_p1: [G.h ? G.h.touch.x : 256, G.h ? G.h.touch.y : 192, G.h ? G.h.touch.w : 0, 0] }),
  common: `
const vec2 C0 = vec2(256., 430.), MC = vec2(256., 172.);
const float MS = .2;
vec3 room(vec2 p, float depth){
  vec3 col; const float HZ = 300.;
  if (p.y > HZ) {
    vec2 w = p - vec2(0., HZ), g = mod(w, vec2(48., 56.)) - vec2(24., 28.); float ang = atan(g.y, g.x);
    col = mixOk(mixOk(u_ground, u_cool, .45), u_ink, .12);
    col = mix(col, mixOk(u_hot, u_ink, .35), step(length(g), 3. + 7.*step(.6, fract(ang*5./6.2832 + .1)))*.45);   // 1960s starbursts
    vec2 wq = p - vec2(120., 640.); float win = sdBox(wq, vec2(72., 92.));
    if (win < 0.) { col = mix(vec3(.5, .18, .1), vec3(.88, .46, .24), fbm(wq*.03 + vec2(u_now*.5, 0.)));          // outside: Mars, and its dust
      col = mix(col, vec3(1., .8, .6), smoothstep(34., 0., length(wq - vec2(18., 40.)))*.6);
      col = mix(col, u_ground*.5, step(abs(wq.x), 2.5) + step(abs(wq.y), 2.5)); }
    col = mix(col, u_ground*.4, 1. - smoothstep(0., 2., abs(win) - 4.));
    vec2 pq = p - vec2(392., 650.); float pf = sdBox(pq, vec2(54., 68.));
    if (pf < 0.) { vec2 q = pq/40. + vec2(0., .2); col = mixOk(u_cool, u_ink, .6);                                   // Perky Pat, framed
      float doll = min(length(q - vec2(0., .95)) - .3, max(abs(q.x) - .42 + q.y*.22, abs(q.y + .2) - .85)); col = mix(col, mixOk(u_hot, u_ground, .15), step(doll, 0.)); }
    col = mix(col, u_ground*.45, 1. - smoothstep(0., 2., abs(pf) - 4.));
    if (depth > 1.5) {                                                                                                  // artificial eyes, open in the wall
      vec2 eq = vec2(abs(p.x - 256.) - 46., p.y - 790.); float eye = length(eq*vec2(1., 2.6)) - 26.;
      vec2 look = normalize(u_p1.xy - vec2(256., 790.) + 1e-3)*7.;
      col = mix(col, mixOk(u_ink, vec3(.7, .75, .8), .5), step(eye, 0.));
      col = mix(col, vec3(.1, .12, .14), step(length(eq - look*vec2(1., .4)), 8.)*step(eye, 0.));
      col = mix(col, u_hot, (1. - smoothstep(0., 2., abs(eye)))*.8); }
    col = mix(col, u_ground*.6, step(p.y, HZ + 14.));
  } else {
    float Z = 340./max(340. - p.y, 1.), X = (p.x - 256.)/60.*Z;                                                         // checked lino in perspective
    col = mix(mixOk(u_ink, u_cool, .25), mixOk(u_ground, u_hot, .25), mod(floor(X) + floor(Z*2.5), 2.))*(.6 + .4/Z);
  }
  float top = sdBox(p - vec2(256., 150.), vec2(130., 32.));                                                              // the card table
  if (top < 0.) col = mixOk(u_hot, u_ground, .45)*(.85 + .15*vnoise(p*vec2(.05, .4)));
  float legs = min(sdBox(p - vec2(146., 70.), vec2(7., 52.)), sdBox(p - vec2(366., 70.), vec2(7., 52.)));
  if (legs < 0. && top > 0.) col = mixOk(u_hot, u_ground, .65);
  if (depth > .5) { float tooth = (p.y - 118.) + 8.*abs(fract(p.x/10.) - .5)*2.;                                        // steel teeth along the table's edge
    if (abs(p.x - 256.) < 130. && p.y < 118. && tooth > 0.) col = mix(vec3(.75, .78, .82), u_ink, step(.5, fract(p.x/10.))*.3); }
  return col;
}
// the little console on the table: its screens are the big screens' last frame
vec3 mini(vec2 p, vec3 col){
  float body = sdBox(p - MC, vec2(256., 430.)*MS + 6.) - 4.;
  col = mix(col, col*.55, (1. - smoothstep(-2., 10., sdBox(p - MC - vec2(5., -6.), vec2(256., 430.)*MS + 6.) - 4.))*.7);
  if (body < 0.) {
    col = mixOk(u_ground, u_ink, .3)*(.8 + .2*smoothstep(-8., 0., body));
    vec2 cp = C0 + (p - MC)/MS;
    if (cp.x > 0. && cp.x < 512.) {
      if (cp.y > 384. + DS_GAP && cp.y < CS.y) col = texture(u_aux, (cp - vec2(0., 384. + DS_GAP))/LS).rgb;
      else if (cp.y > 0. && cp.y < 384.) col = texture(u_aux2, cp/LS).rgb;
    }
  }
  return col;
}
vec3 diorama(vec2 f){
  vec2 C = dsCanvas(f);
  float k = pow(MS, u_p0.x);
  vec2 F = C0 + (MC - C0)/(1. - MS), p = F + (C - F)*k;                    // Can-D: zoom into the little console until it is the big one
  vec3 col = mini(p, room(p, u_p0.y));
  vec2 lamp = vec2(470., 520.); col += mixOk(u_hot, u_ink, .5)*.25*exp(-length(p - lamp)/(160.*k + 1.));
  if (u_p0.y > 2.5 && iScreen == 1 && u_p1.z > .05) {                      // the mechanical hand, where you touch
    vec2 h = (f - u_p1.xy)/16.; float hand = sdBox(h - vec2(0., -.6), vec2(.55, .6)) - .1;
    for (int i = 0; i < 4; i++) hand = min(hand, sdSeg(h, vec2(-.42 + float(i)*.28, 0.), vec2(-.48 + float(i)*.3, .9 + .15*sin(float(i)*2.))) - .1);
    col = mix(col, mix(vec3(.6, .64, .7), u_ink, .3), aa(hand*16.)*u_p1.z); col = mix(col, vec3(.1), aa(abs(hand*16.) - .7)*u_p1.z); }
  return col;
}`,
  glsl: {
    top: `
void mainImage(out vec4 o, in vec2 f){
  vec3 col = diorama(f), U = ui(f);
  col = mix(col, u_ground*.6, smoothstep(.05, .5, uiL(f, 2.).g)*.6);
  col = mix(col, u_ink, smoothstep(.3, .6, U.g));
  o = vec4(col, 1.);
}`,
    bot: `
void mainImage(out vec4 o, in vec2 f){
  vec3 col = diorama(f), U = ui(f);
  col = mix(col, u_ground*.6, smoothstep(.05, .5, uiL(f, 2.).r)*.6);
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  o = vec4(col, 1.);
}`
  }
});
})();
