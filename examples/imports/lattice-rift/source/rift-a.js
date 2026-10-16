/* rift-a.js — rifts 1–3: the title screen inside itself, hit points as a tree, the inventory in hyperbolic space. */
window.RIFTS = window.RIFTS || [];
const RIFT_ATL = `vec3 atl(int i, vec2 l){ float in_ = step(0., l.x)*step(l.x, 1.)*step(0., l.y)*step(l.y, 1.); return texture(iChannel1, (vec2(float(i % 8), float(1 - i/8)) + clamp(l, .002, .998))/vec2(8., 2.)).rgb*in_; }
float sdBox(vec2 p, vec2 b, float r){ vec2 q = abs(p) - b + r; return length(max(q, 0.)) + min(max(q.x, q.y), 0.) - r; }
mat2 rot2(float a){ float c = cos(a), s = sin(a); return mat2(c, -s, s, c); }
vec3 hueRot(vec3 c, float a){ vec3 L = toOklab(c); L.yz = rot2(a)*L.yz; return fromOklab(L); }`;
window.RIFTS.push(

/* ───────────────────────── 01 · the title screen, inside itself ───────────────────────── */
{
  id: 'dissolve', genre: 'Title screen, recursive', edge: 'straight', res: [256, 192], shell: [0.12, 0.04, 0.16],
  pal: { ground: '#0D0221', ink: '#FFF4F8', hot: '#FF2E88', cool: '#2DE2E6' },
  title: 'PRESS START\nTO DISSOLVE', titleCss: "font-family:'Unbounded','Arial Black',sans-serif;font-weight:900;font-size:clamp(34px,6.2vw,96px);line-height:.96",
  lede: 'The title screen draws itself into its own last frame, a little smaller and turned, so it is always inside itself. Behind the console the same picture is cut into a Droste spiral and an eight-way kaleidoscope. Then you start pressing things.',
  try: 'Rub the bottom screen: your ink is fed back into the menu and blooms outward. Then hold any key.',
  how: 'Both screens are feedback simulations (dshost sim): the top one samples its previous frame at 1.16× with a turn, so content recedes; the bottom one samples at 0.93×, so it grows. Each frame rotates the hue in OKLab. The page background maps the top screen through log-polar coordinates with a Droste twist.',
  glsl: {
    echoT: RIFT_ATL + `
void mainImage(out vec4 o, in vec2 f){
  vec2 c = vec2(128., 96.), p = f - c;
  vec2 q = rot2(.04 + .03*sin(iTime*.4))*p*1.16 + c;     // the last frame, smaller and turned: the screen inside the screen
  vec3 col = hueRot(texture(iChannel0, q/vec2(256., 192.)).rgb, .06)*.985;
  float d = sdBox(p, vec2(118., 86.), 10.);
  vec3 rim = .5 + .5*cos(6.283*(iTime*.3 + p.x*.004 + vec3(0., .33, .67)));
  col = mix(col, rim, 1. - smoothstep(2., 3.5, abs(d + 3.)));
  col = mix(col, u_ground, step(1., d));
  vec3 ty = typeAt(f*2.);
  col = mix(col, u_hot*1.3, ty.z*.5*(1. - ty.x));
  col = mix(col, mix(u_ink, u_cool, step(96., f.y + 6.*sin(f.x*.08 + iTime*3.))), ty.x);
  o = vec4(col, 1.);
}`,
    echoB: RIFT_ATL + `
uniform float u_hue;
void mainImage(out vec4 o, in vec2 f){
  vec2 c = vec2(128., 96.), p = f - c;
  vec2 q = rot2(-.03)*p*.93 + c;                         // the last frame, larger: everything blooms outward
  vec3 col = hueRot(texture(iChannel0, q/vec2(256., 192.)).rgb, -.045)*.962;
  vec2 d = f - iTouch.xy*.5;
  col += (.5 + .5*cos(6.283*(u_hue + vec3(0., .33, .67))))*exp(-dot(d, d)/26.)*iTouch.z;
  vec3 ty = typeAt(f*2.);
  col += u_hot*ty.z*.25;
  col = mix(col, .5 + .5*cos(6.283*(iTime*.15 + vec3(0., .33, .67))), ty.x*(.55 + .45*step(0., sin(iTime*6.))));
  o = vec4(col, 1.);
}`,
    show: `
void mainImage(out vec4 o, in vec2 f){
  vec2 uv = f/vec2(512., 384.), ca = (uv - .5)*.012;
  o = vec4(texture(iChannel0, uv + ca).r, texture(iChannel0, uv).g, texture(iChannel0, uv - ca).b, 1.);
}`,
    bg: `
vec3 hueRot(vec3 c, float a){ vec3 L = toOklab(c); L.yz = rot(a)*L.yz; return fromOklab(L); }
void mainImage(out vec4 o, in vec2 f){
  vec2 p = view(f) + vec2(.18, (iScroll - .5)*.4);
  float r = length(p) + 1e-4, a = atan(p.y, p.x), L = log(2.6), t = iTime*.22;
  float z = log(r) - t - a*L/(2.*PI);                    // log-polar with a Droste twist: one spiral of rings
  float k = mod(z, L)/L, ring = floor(z/L);
  float seg = 2.*PI/8., aa = abs(mod(a + t*.5 + ring*.3, seg) - seg*.5)/seg*2.;   // and an eight-way kaleidoscope
  vec3 col = topTex(vec2(aa*.9 + .05, k));
  col = mix(col, botTex(vec2(k, aa)), .3 + .3*sin(ring*1.7 + t*3.));
  col = hueRot(col, ring*.7 + t);
  col *= .35 + .65*smoothstep(0., .06, k)*smoothstep(1., .94, k);
  col *= .4 + .6*smoothstep(0., .25, r);
  col += u_hot*.5*exp(-r*9.);
  col += screenLight(f)*.6;
  vec2 w = f + 7.*u_k*vec2(sin(f.y*.02/u_k + iTime*2.3), cos(f.x*.017/u_k + iTime*1.9));   // the headline as liquid chrome
  vec3 ty = titleAt(w);
  float e = 1.5*u_k;
  vec2 g = vec2(titleAt(w + vec2(e, 0.)).y - titleAt(w - vec2(e, 0.)).y, titleAt(w + vec2(0., e)).y - titleAt(w - vec2(0., e)).y);
  vec3 n = normalize(vec3(-g*4., 1.));
  float v = (fract(titleUV(w).y*2.) - .5)*2. + n.y*.9;
  vec3 env = v > 0. ? mixOk(vec3(1.), u_cool, clamp(v, 0., 1.)) : mixOk(vec3(.15, 0., .2), u_hot, clamp(-v*1.6, 0., 1.));
  env += pow(max(dot(n, normalize(vec3(.4, .6, 1.))), 0.), 24.);
  col = mix(col, u_hot, ty.z*.5*(1. - ty.x));
  col = mix(col, vec3(0.), smoothstep(.15, .45, ty.y)*(1. - ty.x));
  col = mix(col, env, ty.x);
  o = vec4(col, 1.);
}`
  },
  frame(h) {
    const logo = h.typeField('logo', { text: 'LATTICE\nQUEST', size: 62, y: 196, weight: 900, font: "Unbounded, 'Arial Black', sans-serif", lh: 1 });
    const menu = h.typeField('menu', { text: 'PRESS START\nCONTINUE\nDISSOLVE', size: 30, y: 192, weight: 900, font: "Unbounded, 'Arial Black', sans-serif", lh: 1.5 });
    const top = h.sim('echoT', 'echoT', 256, 192, { everyFrame: true, textures: { u_type: logo } });
    const bot = h.sim('echoB', 'echoB', 256, 192, { everyFrame: true, uniforms: { u_hue: (h.time * 0.2) % 1 }, textures: { u_type: menu } });
    h.screen('top', 'show', { textures: { iChannel0: top } });
    h.screen('bottom', 'show', { textures: { iChannel0: bot } });
  }
},

/* ───────────────────────── 02 · hit points, all the way down ───────────────────────── */
{
  id: 'hp', genre: 'Health bar, recursive', edge: 'pixel', res: [256, 192], shell: [0.05, 0.1, 0.06],
  pal: { ground: '#070C08', ink: '#F0FFF4', hot: '#FF2D55', cool: '#3CFF7A' },
  title: 'HIT POINTS\nALL THE WAY\nDOWN', titleCss: "font-family:'Big Shoulders Display','Arial Narrow',sans-serif;font-weight:900;font-size:clamp(50px,8vw,124px);line-height:.86",
  lede: 'A health bar that branches. Each bar splits into two smaller bars, nine times, and each level drains on its own clock, so the tree is always dying somewhere and full somewhere else. The number below is made of the number.',
  try: 'Hold the bottom screen to heal; the digits near your finger scramble.',
  how: 'The tree is a 2D fold: draw a bar, mirror x, move up, turn, shrink by 0.72, repeat nine times, so 512 bars cost nine iterations per pixel. The digits come from a glyph atlas; each 9 px cell of the big number shows a small copy of its own digit.',
  glsl: {
    tree: RIFT_ATL + `
uniform float u_hp;
float sdHeart(vec2 p){ p.x = abs(p.x); if (p.y + p.x > 1.) return length(p - vec2(.25, .75)) - .3536;
  return sqrt(min(dot(p - vec2(0., 1.), p - vec2(0., 1.)), dot(p - .5*max(p.x + p.y, 0.), p - .5*max(p.x + p.y, 0.))))*sign(p.x - p.y); }
void mainImage(out vec4 o, in vec2 f){
  vec2 p = f - vec2(256., 14.);
  vec3 col = u_ground + u_cool*.05*(1. - f.y/384.);
  float s = 1., ang = .5 + .14*sin(iTime*.6);
  for (int i = 0; i < 9; i++) {
    vec2 q = p - vec2(0., 36.);
    float d = sdBox(q, vec2(8., 34.), 4.)*s;
    float fill = fract(u_hp + float(i)*.17 - iTime*.12*(1. + float(i)*.25));
    float yy = (q.y + 34.)/68.;
    vec3 bc = yy < fill ? mixOk(u_hot, u_cool, fill) : u_ground*2.5;
    bc += step(abs(yy - fill), .04)*.6;
    col = mix(col, bc, 1. - smoothstep(-.6, .6, d));
    col = mix(col, u_ink*.8, (1. - smoothstep(.4, 1.2, abs(d)))*.7);
    p.y -= 70.; p.x = abs(p.x); p = rot2(-ang)*p; p /= .72; s *= .72;
  }
  float beat = 1. + .15*pow(abs(sin(iTime*3.)), 8.);
  float hd = sdHeart((p - vec2(0., -8.))/(34.*beat) + vec2(0., .5))*34.*beat*s;
  col = mix(col, mixOk(u_hot, vec3(1.), .2), 1. - smoothstep(-.6, .6, hd));
  col += u_hot*.25*exp(-max(hd, 0.)/4.);
  o = vec4(col, 1.);
}`,
    digits: RIFT_ATL + `
uniform float u_val, u_heal;
float digitAt(float i){ return floor(mod(u_val/pow(10., 3. - i), 10.)); }
float big(vec2 c, out float which){
  which = clamp(floor((c.x - 0.)/128.), 0., 3.);
  vec2 l = (c - vec2(64. + which*128., 196.))/300. + .5;
  return atl(int(digitAt(which)), l).x;
}
void mainImage(out vec4 o, in vec2 f){
  float cs = 9.; vec2 id = floor(f/cs), cc = (id + .5)*cs, lc = fract(f/cs);
  float which; float cov = big(cc, which);
  float dv = digitAt(which);
  vec2 dt = cc - iTouch.xy; float near = exp(-dot(dt, dt)/3000.)*iTouch.w;
  if (near > hash12(id + floor(iTime*20.))) dv = floor(hash12(id + iTime)*10.);
  float m = atl(int(dv), (lc - .5)*1.25 + .5).x;
  vec3 hot = mixOk(u_hot, u_cool, u_val/9999.);
  vec3 col = u_ground + hot*m*(cov > .5 ? 1. : .14) + u_cool*u_heal*.15*m;
  col += hot*.3*smoothstep(.5, 1., cov)*(1. - m);
  o = vec4(col, 1.);
}`,
    bg: `
float sdHeart(vec2 p){ p.x = abs(p.x); if (p.y + p.x > 1.) return length(p - vec2(.25, .75)) - .3536;
  return sqrt(min(dot(p - vec2(0., 1.), p - vec2(0., 1.)), dot(p - .5*max(p.x + p.y, 0.), p - .5*max(p.x + p.y, 0.))))*sign(p.x - p.y); }
uniform float u_val;
void mainImage(out vec4 o, in vec2 f){
  vec2 p = sect(f)*3.2 + vec2(0., iTime*.15);
  vec3 col = u_ground;
  float beat = pow(abs(sin(iTime*3.)), 8.);
  for (int lvl = 0; lvl < 4; lvl++) {                 // hearts inside hearts inside hearts
    vec2 id = floor(p), c = fract(p) - .5;
    float hs = hash12(id + float(lvl)*7.);
    float d = sdHeart((c + vec2(0., .38))*(1.9 - .2*beat*step(.5, hs)));
    if (d > 0.) { col += u_cool*.06*exp(-d*8.)*(1. + hs); break; }
    col = mix(col, topTex(c*1.4 + .5)*(.6 + .4*hs), .7);
    col = mix(col, u_hot, smoothstep(.04, 0., abs(d))*.8);
    p = c*4. + vec2(hs*10., iTime*.2);
  }
  col += screenLight(f)*.6;
  vec3 ty = titleAt(f);                              // the headline as a health bar draining
  vec2 tu = titleUV(f);
  float fill = .15 + .7*u_val/9999.;
  vec3 bar = tu.x < fill ? mixOk(u_hot, u_cool, tu.x/max(fill, .01)) : u_ground*3.;
  bar += .5*step(abs(tu.x - fill), .006);
  col = mix(col, u_hot*.8, ty.z*.4*(1. - ty.x));
  col = mix(col, vec3(0.), smoothstep(.1, .4, ty.y)*(1. - ty.x));
  col = mix(col, bar, ty.x);
  o = vec4(col, 1.);
}`
  },
  frame(h) {
    const s = h.state, K = window.GAMEKIT;
    if (s.val == null) { s.val = 9999; s.heal = 0; }
    s.heal += ((h.touch.down ? 1 : 0) - s.heal) * Math.min(1, h.dt * 5);
    s.val += (h.touch.down ? 2600 : -380 - 300 * Math.sin(h.time * 0.7)) * h.dt;
    if (s.val < 0) s.val = 9999; s.val = Math.min(9999, s.val);
    const font = "'Big Shoulders Display', 'Arial Narrow', sans-serif";
    const atlas = K.atlas(h, 'digits', [...'0123456789'].map(d => [d, font, 104]));
    h.screen('top', 'tree', { uniforms: { u_hp: s.val / 9999 }, textures: { iChannel1: atlas } });
    h.screen('bottom', 'digits', { uniforms: { u_val: Math.floor(s.val), u_heal: s.heal }, textures: { iChannel1: atlas } });
    h.bgUniforms = { u_val: s.val };
  }
},

/* ───────────────────────── 03 · the infinite inventory ───────────────────────── */
{
  id: 'satchel', genre: 'Inventory, hyperbolic', edge: 'tear', res: [256, 192], shell: [0.09, 0.06, 0.14],
  pal: { ground: '#0E0A1F', ink: '#F8F4FF', hot: '#FFB800', cool: '#9B5CFF' },
  title: 'INFINITE\nINVENTORY', titleCss: "font-family:'Syne','Arial Black',sans-serif;font-weight:800;font-size:clamp(34px,6.2vw,98px);line-height:.92",
  lede: 'An inventory with infinitely many slots, all of them within reach: the grid is laid out in hyperbolic space, so the slots shrink toward the rim of a disk forever. Each slot holds an item folded into a kaleidoscope. The bottom screen is a satchel whose every pocket holds the top screen.',
  try: 'Drag on the bottom screen to slide the disk through hyperbolic space.',
  how: 'The {4,6} tiling: reflect the point into one triangle of the tiling (a line, a second line at 45°, and a circle orthogonal to the rim), counting reflections. The circle inversions count rings, which picks the item. Panning is a Möbius transform. The bottom grid maps x ↦ x ÷ (1 − |x|), so its slots also run on forever.',
  glsl: {
    disk: RIFT_ATL + `
uniform vec2 u_pan; uniform float u_rot;
vec2 cmul(vec2 a, vec2 b){ return vec2(a.x*b.x - a.y*b.y, a.x*b.y + a.y*b.x); }
vec2 cdiv(vec2 a, vec2 b){ return cmul(a, vec2(b.x, -b.y))/dot(b, b); }
void mainImage(out vec4 o, in vec2 f){
  vec2 p = (f - vec2(256., 192.))/184.;
  float r = length(p);
  vec3 col = u_ground + u_cool*.08*exp(-abs(r - 1.)*10.);
  if (r < 1.) {
    vec2 z = rot2(u_rot)*p;
    z = cdiv(z + u_pan, vec2(1., 0.) + cmul(vec2(u_pan.x, -u_pan.y), z));       // Möbius: slide through the plane
    float a = PI/4., b = PI/6., den = sqrt(cos(b)*cos(b) - sin(a)*sin(a));
    float x0 = cos(b)/den, rr = sin(a)/den, inv = 0., refl = 0.;
    vec2 nA = vec2(-sin(a), cos(a));
    for (int i = 0; i < 48; i++) {
      bool ch = false;
      if (z.y < 0.) { z.y = -z.y; refl++; ch = true; }
      float dA = dot(z, nA); if (dA > 0.) { z -= 2.*dA*nA; refl++; ch = true; }
      vec2 w = z - vec2(x0, 0.); float l2 = dot(w, w);
      if (l2 < rr*rr) { z = vec2(x0, 0.) + w*rr*rr/l2; inv++; ch = true; }
      if (!ch) break;
    }
    float edge = abs(length(z - vec2(x0, 0.)) - rr);
    float kind = mod(inv, 3.);
    vec3 slot = mixOk(u_ground*3., kind < 1. ? u_cool : kind < 2. ? vec3(.3, .7, 1.) : u_hot, .35 + .25*(1. - length(z)*2.));
    float ic = kind < 1. ? (abs(z.x)*1.5 + abs(z.y)) - .2 : kind < 2. ? length(z - vec2(.02, .0)) - .14 : abs(length(z) - .13) - .035;
    slot = mix(slot, mix(u_ink, kind < 2. ? u_cool : u_hot, .3), 1. - smoothstep(-.004, .004, ic));
    if (inv < .5) slot += u_hot*.25*(.5 + .5*sin(iTime*5.));
    float fw = fwidth(edge) + 1e-4;
    slot = mix(slot, u_hot, 1. - smoothstep(.006, .006 + fw*1.5, edge));
    col = mix(slot, col, smoothstep(.985, 1., r));
  }
  o = vec4(col, 1.);
}`,
    pockets: RIFT_ATL + `
uniform vec2 u_scroll;
void mainImage(out vec4 o, in vec2 f){
  vec2 n = (f - vec2(256., 192.))/vec2(256., 192.);
  vec2 g = n/(1. - abs(n)*.985)*1.6 + u_scroll;                       // pockets run on forever toward the edges
  vec2 id = floor(g), l = fract(g);
  float d = sdBox(l - .5, vec2(.43), .1);
  vec3 inner = texture(iChannel1, clamp((l - .1)/.8, 0., 1.)).rgb;       // every pocket holds the top screen
  inner = hueRot(inner, hash12(id)*6.283*.3);
  vec2 tn = (iTouch.xy - vec2(256., 192.))/vec2(256., 192.);
  vec2 tg = floor(tn/(1. - abs(tn)*.985)*1.6 + u_scroll);
  bool on = all(equal(tg, id)) && iTouch.w > .05;
  float fw = fwidth(d) + 1e-4;
  vec3 col = mix(u_ground, inner*(on ? 1.3 : .85), 1. - smoothstep(-fw, fw, d));
  col = mix(col, on ? u_hot : mixOk(u_cool, u_hot, hash12(id + 3.)), 1. - smoothstep(fw, 2.5*fw, abs(d)));
  col *= 1. - .6*pow(max(abs(n.x), abs(n.y)), 6.);
  o = vec4(col, 1.);
}`,
    bg: `
vec2 cmul(vec2 a, vec2 b){ return vec2(a.x*b.x - a.y*b.y, a.x*b.y + a.y*b.x); }
vec2 cdiv(vec2 a, vec2 b){ return cmul(a, vec2(b.x, -b.y))/dot(b, b); }
void mainImage(out vec4 o, in vec2 f){
  vec2 p = sect(f)*1.9 + vec2(.0, .05);
  float r = length(p);
  vec3 col = u_ground*(1. + .5*fbm(p*4. + iTime*.05));
  vec2 m = mouseView()*.4;
  if (r < 1.) {
    vec2 a0 = vec2(.3*sin(iTime*.11), .3*cos(iTime*.07)) + m*.5;
    vec2 z = cdiv(rot(iTime*.05)*p + a0, vec2(1., 0.) + cmul(vec2(a0.x, -a0.y), rot(iTime*.05)*p));
    float a = PI/5., b = PI/4., den = sqrt(cos(b)*cos(b) - sin(a)*sin(a)), x0 = cos(b)/den, rr = sin(a)/den, inv = 0.;
    vec2 nA = vec2(-sin(a), cos(a));
    for (int i = 0; i < 40; i++) { bool ch = false;
      if (z.y < 0.) { z.y = -z.y; ch = true; }
      float dA = dot(z, nA); if (dA > 0.) { z -= 2.*dA*nA; ch = true; }
      vec2 w = z - vec2(x0, 0.); float l2 = dot(w, w); if (l2 < rr*rr) { z = vec2(x0, 0.) + w*rr*rr/l2; inv++; ch = true; }
      if (!ch) break; }
    float edge = abs(length(z - vec2(x0, 0.)) - rr);
    vec3 tile = botTex(z*1.6 + .5)*(.55 + .45*mod(inv, 2.));
    tile = mix(tile, mixOk(u_cool, u_hot, mod(inv, 3.)/2.), .25);
    tile = mix(tile, u_hot, 1. - smoothstep(.005, .005 + 1.5*fwidth(edge) + 1e-4, edge));
    col = mix(tile, col, smoothstep(.99, 1., r));
  }
  col += u_hot*.6*exp(-abs(r - 1.)*60.);
  col += screenLight(f)*.5;
  vec2 mo = iMouse.xy, dm = f - mo; float R = 180.*u_k, l = length(dm);   // the headline under a hyperbolic lens that follows the mouse
  vec2 w = mo + dm*mix(.55, 1., smoothstep(0., R, l));
  vec3 ty = titleAt(w);
  vec2 tu = titleUV(w);
  vec3 foil = mix(vec3(.6, .4, .1), vec3(1., .9, .6), .5 + .5*sin(tu.y*12. + tu.x*3.)) + .35*(.5 + .5*cos(6.283*(tu.x - tu.y + iTime*.2 + vec3(0., .33, .67))));
  col = mix(col, u_cool*.6, ty.z*.4*(1. - ty.x));
  col = mix(col, foil, ty.x);
  o = vec4(col, 1.);
}`
  },
  frame(h) {
    const s = h.state, t = h.touch;
    if (!s.pan) { s.pan = [0, 0]; s.vel = [0.02, 0.01]; s.scroll = [0, 0]; s.last = null; }
    if (t.down) {
      if (s.last) { const dx = (t.x - s.last[0]) / 400, dy = (t.y - s.last[1]) / 400; s.vel = [dx / Math.max(h.dt, 1e-3) * 0.3, dy / Math.max(h.dt, 1e-3) * 0.3]; s.scroll[0] -= dx * 6; s.scroll[1] -= dy * 6; }
      s.last = [t.x, t.y];
    } else { s.last = null; s.vel = s.vel.map(v => v * Math.exp(-h.dt * 0.6)); s.scroll[0] += h.dt * 0.25; }
    s.pan[0] += s.vel[0] * h.dt; s.pan[1] += s.vel[1] * h.dt;
    const m = Math.hypot(s.pan[0], s.pan[1]); if (m > 0.92) { s.pan = s.pan.map(v => -v * 0.2); }
    h.screen('top', 'disk', { uniforms: { u_pan: s.pan, u_rot: h.time * 0.07 } });
    h.screen('bottom', 'pockets', { uniforms: { u_scroll: s.scroll }, textures: { iChannel1: h.scr[0] || h.H.white } });
  }
}
);
