/* rift-b.js — rifts 4–5: the map that eats itself, the dialogue that answers itself. */
window.RIFTS = window.RIFTS || [];
const RIFT_JULIA = `
vec4 julia(vec2 z, vec2 c){ float n = 0., trap = 1e9;
  for (int i = 0; i < 64; i++) { z = vec2(z.x*z.x - z.y*z.y, 2.*z.x*z.y) + c; trap = min(trap, abs(z.x) + abs(z.y)*.5); if (dot(z, z) > 16.) break; n += 1.; }
  return vec4(n - log2(max(log2(max(dot(z, z), 1.0001)), 1e-4)) + 4., trap, n, 0.); }
// a sea chart of the set: land inside, depth bands outside, an engraved coast
vec3 chart(vec4 j, float fw){
  bool land = j.z > 63.;
  vec3 sea = mixOk(u_cool*.75, u_ground*1.6, clamp(j.x/24., 0., 1.));
  sea = mix(sea, u_ink*.7, .35*(1. - smoothstep(0., max(fw, 1e-3)*1.5, abs(fract(j.x*.5) - .5))));
  vec3 ground = mixOk(vec3(.86, .74, .5), vec3(.25, .5, .3), smoothstep(.0, .8, j.y));
  ground = mix(ground, u_ground, .45*step(.5, fract(j.y*7.)));
  return mix(land ? ground : sea, u_ink, .85*clamp(fwidth(land ? 1. : 0.), 0., 1.));
}`;
window.RIFTS.push(

/* ───────────────────────── 04 · the map that eats itself ───────────────────────── */
{
  id: 'julia', genre: 'World map, Julia set', edge: 'wave', res: [256, 192], shell: [0.05, 0.1, 0.12],
  pal: { ground: '#06121F', ink: '#F5EBD8', hot: '#FF5A3C', cool: '#36D1C4' },
  title: 'THE MAP\nEATS ITSELF', titleCss: "font-family:'Instrument Serif',Georgia,serif;font-style:italic;font-size:clamp(64px,10.5vw,168px);line-height:.86",
  lede: 'The world map is a Julia set: every coastline is the boundary of the points that do not run away under z ↦ z² + c. Change c and the continents breathe, split into archipelagos and fall into dust. The top screen flies down a tunnel lined with the same coast.',
  try: 'Drag on the bottom screen to move c. Near the edge of the Mandelbrot set the land is most alive.',
  how: 'Sixty-four iterations per pixel, smoothed escape time for the sea depths and an orbit trap for the land bands. The tunnel maps angle and 1/radius onto a mirrored repeat of the same set. The page background is the Mandelbrot set, the map of all these maps, with a ring where your c is.',
  glsl: {
    tunnel: RIFT_JULIA + `
uniform vec2 u_c;
void mainImage(out vec4 o, in vec2 f){
  vec2 p = (f - vec2(256., 192.))/192.;
  float r = length(p), a = atan(p.y, p.x);
  vec2 t = vec2(a/PI*1.5, .4/r + iTime*.4);
  vec2 z = (1. - abs(1. - mod(t, 2.)))*2.6 - 1.3;
  vec4 j = julia(z, u_c);
  vec3 col = chart(j, fwidth(j.x));
  col *= smoothstep(0., .45, r);
  col += u_hot*.7*exp(-r*7.);
  o = vec4(col, 1.);
}`,
    chart: RIFT_JULIA + `
uniform vec2 u_c;
void mainImage(out vec4 o, in vec2 f){
  vec2 z = (f - vec2(256., 192.))/150.;
  vec4 j = julia(z, u_c);
  vec3 col = chart(j, fwidth(j.x));
  vec2 g = abs(fract(f/64. - .5) - .5)*64.;
  col = mix(col, u_hot, .2*(1. - smoothstep(0., 1., min(g.x, g.y))));
  vec2 d = f - iTouch.xy; col = mix(col, u_hot, (1. - smoothstep(1., 2.5, abs(length(d) - 10.)))*iTouch.w);
  o = vec4(col, 1.);
}`,
    bg: RIFT_JULIA + `
uniform vec2 u_c;
void mainImage(out vec4 o, in vec2 f){
  float zoom = 2.2*exp(iScroll*3.2);                                     // scroll down: dive into seahorse valley
  vec2 ctr = vec2(-.7436, .1318);
  vec2 c = ctr + sect(f)*2.7/zoom + vec2(.05*sin(iTime*.1), 0.);
  vec2 z = vec2(0.); float n = 0.;
  for (int i = 0; i < 56; i++) { z = vec2(z.x*z.x - z.y*z.y, 2.*z.x*z.y) + c; if (dot(z, z) > 16.) break; n += 1.; }
  float sm = n - log2(max(log2(max(dot(z, z), 1.0001)), 1e-4));
  vec3 col = n > 55. ? u_ground*1.5 + vec3(.86, .74, .5)*.22*smoothstep(.6, 1., sin(length(z)*24.)) : mixOk(u_cool*.6, u_ground, clamp(sm/30., 0., 1.));
  col = mix(col, u_ink*.6, .3*(1. - smoothstep(0., fwidth(sm)*1.5 + 1e-3, abs(fract(sm*.5) - .5))));
  float rc = length((c - u_c)*zoom*iStage.y/(2.7*u_k));                // a ring where your c is
  col = mix(col, u_hot, 1. - smoothstep(1., 2.5, abs(rc - 14. - 4.*sin(iTime*5.))));
  col += screenLight(f)*.5;
  vec3 ty = titleAt(f);                                                   // the headline, eaten by the current set
  vec4 jt = julia((titleUV(f) - .5)*vec2(3.2, 1.6), u_c);
  float eaten = jt.z > 63. ? 1. : smoothstep(6., 2., jt.x);
  col = mix(col, u_hot*.6, ty.z*.35*(1. - ty.x));
  col = mix(col, mix(u_ink, u_hot, eaten*.8), ty.x*(1. - eaten*.6));
  o = vec4(col, 1.);
}`
  },
  frame(h) {
    const s = h.state, t = h.touch;
    if (!s.c) s.c = [-0.4, 0.6];
    const want = t.down ? [(t.x - 256) / 256 * 1.1 - 0.35, (t.y - 192) / 192 * 0.85] : [0.7885 * Math.cos(h.time * 0.23), 0.7885 * Math.sin(h.time * 0.23)];
    const k = Math.min(1, h.dt * (t.down ? 6 : 0.8));
    s.c[0] += (want[0] - s.c[0]) * k; s.c[1] += (want[1] - s.c[1]) * k;
    h.screen('top', 'tunnel', { uniforms: { u_c: s.c } });
    h.screen('bottom', 'chart', { uniforms: { u_c: s.c } });
    h.bgUniforms = { u_c: s.c };
  }
},

/* ───────────────────────── 05 · the dialogue answers itself ───────────────────────── */
{
  id: 'yesno', genre: 'Dialogue, branching forever', edge: 'glitch', res: [256, 192], shell: [0.1, 0.1, 0.1],
  pal: { ground: '#0B0B0B', ink: '#F6F6F0', hot: '#FF3B1F', cool: '#3D5AFE' },
  title: 'YES / NO /\nYES / NO', titleCss: "font-family:'Rubik Mono One','Arial Black',sans-serif;font-size:clamp(40px,7vw,108px);line-height:1.05",
  lede: 'Every dialogue tree is a choice between two buttons. Here they grow like seeds in a sunflower, YES and NO alternating, a hundred and forty of them turning in a golden-angle spiral, and the face on the top screen is made of letters. It keeps asking.',
  try: 'Tap a button on the bottom screen. The face hears you and asks again.',
  how: 'Phyllotaxis: button k sits at angle k·137.5° and radius √k, which packs them without gaps. The face is a signed-distance portrait sampled once per 8 px cell, its brightness picking one of ten glyphs (" .:-=+*#%@") from an atlas. The page background is the same glyphs in a log-polar spiral that falls inward forever.',
  glsl: {
    face: RIFT_ATL + `
uniform float u_talk, u_rev; uniform vec2 u_look;
float face(vec2 p){
  vec2 q = (p - vec2(256., 200.))/vec2(118., 150.);
  float head = 1. - length(q);
  if (head < 0.) return .06 + .05*sin(p.y*.05 + iTime*2.);
  vec3 n = normalize(vec3(q, sqrt(max(head*2. - head*head, 0.))));
  float b = .25 + .75*max(dot(n, normalize(vec3(-.5, .4, .8))), 0.);
  for (int s = 0; s < 2; s++) { vec2 e = p - vec2(s == 0 ? 210. : 302., 236.);
    b = mix(b, .0, 1. - smoothstep(18., 20., length(e*vec2(1., 1.5))));
    b = mix(b, 1., 1. - smoothstep(5., 7., length(e - u_look*8.)));
    b = mix(b, .1, (1. - smoothstep(2., 4., abs(e.y - 30. + 4.*sin(iTime*2.))))*step(abs(e.x), 24.)); }   // brows
  float mouth = length((p - vec2(256., 130.))*vec2(1., 1./(.25 + 1.5*u_talk*abs(sin(iTime*17.))))) - 34.;
  b = mix(b, .02, 1. - smoothstep(0., 2., mouth));
  return b;
}
void mainImage(out vec4 o, in vec2 f){
  float cs = 16.; vec2 id = floor(f/cs), l = fract(f/cs);   // 8 × 8 DS pixels a character
  float b = clamp(face((id + .5)*cs) + (hash12(id + floor(iTime*12.)) - .5)*.12, 0., .999);
  float g = min(1., 1.6*atl(2 + int(b*10.), (l - .5)*1.08 + .5).x);
  vec3 c = mixOk(u_cool, u_hot, b)*(1. + .3*sin(id.y*.6 - iTime*6.));
  vec3 col = u_ground + c*(g + b*.12);
  vec3 ty = typeAt(f);
  float box = step(f.y, 46.)*step(4., f.y);
  col = mix(col, u_ground, box*.85);
  col = mix(col, u_ink, ty.x*step(f.x, 24. + u_rev));
  o = vec4(col, 1.);
}`,
    seeds: RIFT_ATL + `
uniform float u_spin, u_hitT; uniform vec2 u_hit;
void mainImage(out vec4 o, in vec2 f){
  vec3 col = u_ground + u_cool*.06*sin(length(f - vec2(256., 192.))*.05 - iTime*3.);
  for (int k = 0; k < 140; k++) {
    float fk = float(k), ang = fk*2.39996 + u_spin, rad = 14.5*sqrt(fk);
    vec2 c = vec2(256., 192.) + rad*vec2(cos(ang)*1.3, sin(ang));
    float s = .3 + fk*.0036;
    vec2 q = rot2(-ang)*(f - c)/s;
    if (abs(q.x) > 56. || abs(q.y) > 24.) continue;
    float d = sdBox(q, vec2(46., 18.), 18.)*s;
    bool yes = mod(fk, 2.) < 1.;
    float hit = exp(-length(c - u_hit)/30.)*exp(-u_hitT*3.);
    vec3 pill = mix(yes ? u_hot : u_cool, vec3(1.), hit);
    float lab = atl(yes ? 0 : 1, q/110. + .5).x;
    col = mix(col, pill, 1. - smoothstep(-.6, .6, d));
    col = mix(col, u_ink, lab*(1. - smoothstep(-.6, .6, d)));
    col = mix(col, u_ground, (1. - smoothstep(.3, 1., abs(d)))*.8);
  }
  o = vec4(col, 1.);
}`,
    bg: RIFT_ATL + `
uniform float u_spin;
void mainImage(out vec4 o, in vec2 f){
  vec2 p = view(f) - vec2(-.25, 0.)*(1. - iScroll);
  float r = length(p) + 1e-4, a = atan(p.y, p.x);
  vec2 lp = vec2(log(r)*5. - iTime*1.1, a/(2.*PI)*22. + log(r)*2.5 + u_spin*.3);   // glyphs falling inward forever
  vec2 id = floor(lp), l = fract(lp);
  float hs = hash12(id);
  int gi = hs < .35 ? 0 : hs < .7 ? 1 : 2 + int(hash12(id + 5.)*10.);
  float g = atl(gi, (l - .5)*vec2(gi < 2 ? .6 : 1., 1.)*1.2 + .5).x;
  float pulse = .5 + .5*sin(id.x*.9 + iTime*4. + hs*6.);
  vec3 c = gi == 0 ? u_hot : gi == 1 ? u_cool : u_ink*.6;
  vec3 col = u_ground + c*g*(.25 + .75*pulse)*smoothstep(.02, .2, r);
  col += u_hot*exp(-abs(r - .05)*80.);
  col += screenLight(f)*.5;
  float sl = floor(f.y/(14.*u_k));                                       // the headline, glitching in slices
  float jx = step(.82, hash12(vec2(sl, floor(iTime*9.))))*(hash12(vec2(sl, 3.)) - .5)*60.*u_k;
  vec2 w = f + vec2(jx, 0.);
  vec3 ty = titleAt(w);
  float cell = floor(titleUV(w).x*11.) + floor(titleUV(w).y*2.);
  vec3 fill = mod(cell, 2.) < 1. ? u_hot : u_ink;
  fill = mix(fill, u_cool, step(.8, hash12(vec2(cell, floor(iTime*3.)))));
  col = mix(col, vec3(0.), smoothstep(.1, .4, ty.y)*(1. - ty.x));
  col = mix(col, fill, ty.x);
  col.r = mix(col.r, titleAt(w + vec2(4.*u_k, 0.)).x, step(.001, abs(jx))*.6);
  o = vec4(col, 1.);
}`
  },
  frame(h) {
    const s = h.state, K = window.GAMEKIT;
    const LINES = ['ARE YOU SURE?', 'ARE YOU SURE YOU ARE SURE?', 'THEN WHY DID YOU SAY NO?', 'YES MEANS NO NOW.', 'ANSWER IN THE FORM OF A QUESTION.', 'WE HAVE BEEN HERE BEFORE.'];
    if (s.line == null) Object.assign(s, { line: 0, rev: 0, spin: 0, sv: 0.15, hit: [0, 0], hitT: 9 });
    const mono = "'JetBrains Mono', 'IBM Plex Mono', monospace";
    const w = K.measure(LINES[s.line], mono, 15, 700);
    if (h.touch.tapped) { s.hit = [h.touch.x, h.touch.y]; s.hitT = 0; s.sv += 2.5; s.line = (s.line + 1) % LINES.length; s.rev = 0; }
    s.rev = Math.min(w + 4, s.rev + h.dt * 260);
    s.sv += (0.15 - s.sv) * Math.min(1, h.dt * 0.8); s.spin += s.sv * h.dt; s.hitT += h.dt;
    const atlas = K.atlas(h, 'glyphs', [['YES', "'Rubik Mono One', sans-serif", 40], ['NO', "'Rubik Mono One', sans-serif", 46], ...' .:-=+*#%@'.split('').map(c => [c, mono, 118, 800])]);
    const line = h.typeField('line:' + s.line, { text: LINES[s.line], size: 15, x: 24, y: 25, align: 'left', weight: 700, font: mono });
    const look = [Math.max(-1, Math.min(1, (h.touch.x - 256) / 200)), Math.max(-1, Math.min(1, (h.touch.y - 192) / 200))];
    h.screen('top', 'face', { uniforms: { u_talk: s.rev < w ? 1 : 0, u_rev: s.rev, u_look: look }, textures: { iChannel1: atlas, u_type: line } });
    h.screen('bottom', 'seeds', { uniforms: { u_spin: s.spin, u_hit: s.hit, u_hitT: s.hitT }, textures: { iChannel1: atlas } });
    h.bgUniforms = { u_spin: s.spin }; h.bgTextures = { iChannel1: atlas };
  }
}
);
