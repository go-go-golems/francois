/* rift-c.js — rifts 6–7: game over grows into a garden, and the screen where you draw the strands. */
window.RIFTS = window.RIFTS || [];
const RIFT_INK = `
vec2 curl(vec2 p){ float e = .1; return vec2(vnoise(p + vec2(0., e)) - vnoise(p - vec2(0., e)), vnoise(p - vec2(e, 0.)) - vnoise(p + vec2(e, 0.)))/(2.*e); }
float sdSeg(vec2 p, vec2 a, vec2 b){ vec2 pa = p - a, ba = b - a; return length(pa - ba*clamp(dot(pa, ba)/max(dot(ba, ba), 1e-4), 0., 1.)); }
vec3 film(float t){ return .5 + .5*cos(6.283*(t + vec3(0., .33, .67))); }`;
window.RIFTS.push(

/* ───────────────────────── 06 · game over is a garden ───────────────────────── */
{
  id: 'garden', genre: 'Game over, reaction–diffusion', edge: 'zigzag', res: [256, 192], shell: [0.05, 0.09, 0.07],
  pal: { ground: '#0A0F0D', ink: '#F4FFE9', hot: '#FF4F79', cool: '#8BFFB0' },
  title: 'GAME OVER\nIS A GARDEN', titleCss: "font-family:'Climate Crisis','Arial Black',sans-serif;font-size:clamp(36px,6.4vw,100px);line-height:1",
  lede: 'Game over is not the end of anything here. The words are planted on the top screen and grow a coral that crosses the hinge, fills both screens and is mown down in clearings by a slow wind, so it is never finished. Each number of the continue countdown is planted too.',
  try: 'Hold the bottom screen to plant. Things grow out of your finger.',
  how: 'A Gray–Scott reaction–diffusion: two chemicals, A feeding in and B eating A, run ten steps a frame on one grid that spans both screens and the hinge. The feed and kill rates drift across the grid, so mazes, coral and spots grow side by side. The display reads B, lights its gradient like an embossed surface and colours new growth pink and old growth green.',
  glsl: {
    rd: `
uniform float u_seed, u_seedB;
void mainImage(out vec4 o, in vec2 f){
  vec2 L = f*2.;                                         // a sim cell → the two-screen canvas (logical px, y-up)
  vec4 c = prev(f);
  if (iFrame == 0) c = vec4(1., 0., 0., 0.);
  vec4 n = prev(f + vec2(0., 1.)) + prev(f - vec2(0., 1.)) + prev(f + vec2(1., 0.)) + prev(f - vec2(1., 0.));
  vec4 g = prev(f + vec2(1.)) + prev(f - vec2(1.)) + prev(f + vec2(1., -1.)) + prev(f + vec2(-1., 1.));
  vec2 lap = iFrame == 0 ? vec2(0.) : -c.xy + .2*n.xy + .05*g.xy;
  vec2 q = L/dsCanvasSize();
  float m = .5 + .5*sin(q.x*5. + q.y*7. + vnoise(q*3.)*4. + iTime*.05);   // the feed and kill rates drift: mazes here, coral there
  float F = mix(.029, .055, m), k = mix(.057, .062, m);
  float A = c.x, B = c.y, r = A*B*B;
  A += lap.x - r + F*(1. - A);
  B += .5*lap.y + r - (k + F)*B;
  B = max(B, typeAt(L - vec2(0., 384. + DS_GAP)).x*u_seed);                 // GAME OVER sows itself on the top screen
  B = max(B, texture(iChannel1, L/vec2(512., 384.)).x*u_seedB*step(L.y, 384.));   // and each countdown digit on the bottom
  vec2 d = L - iTouch.xy; B = max(B, iTouch.z*step(dot(d, d), 110.));
  B *= 1. - .08*smoothstep(.8, .86, vnoise(L*.006 + vec2(iTime*.031, -iTime*.017)));   // a wind that mows clearings
  o = vec4(clamp(A, 0., 1.), clamp(B, 0., 1.), c.z*.985 + B*.015, 1.);    // z: a slow memory of B, the age of the growth
}`,
    bloom: `
void mainImage(out vec4 o, in vec2 f){
  vec2 uv = dsCanvas(f)/dsCanvasSize(), px = 1./vec2(textureSize(iChannel1, 0));
  vec4 s = texture(iChannel1, uv);
  float bx = texture(iChannel1, uv + vec2(px.x, 0.)).y - texture(iChannel1, uv - vec2(px.x, 0.)).y;
  float by = texture(iChannel1, uv + vec2(0., px.y)).y - texture(iChannel1, uv - vec2(0., px.y)).y;
  vec3 n = normalize(vec3(-bx, -by, .22));
  float t = smoothstep(.06, .32, s.y), age = smoothstep(.05, .2, s.z);
  vec3 tint = mixOk(mixOk(u_hot, u_ink, t*.5), mixOk(u_cool*.45, u_cool, t), age);
  float dif = max(dot(n, normalize(vec3(-.5, .6, .65))), 0.), spec = pow(max(dot(reflect(-normalize(vec3(-.5, .6, .65)), n), vec3(0., 0., 1.)), 0.), 18.);
  vec3 col = mix(u_ground + u_cool*.04*s.z, tint*(.35 + .85*dif) + spec*.5, t);
  if (iScreen == 0) { vec3 ty = typeAt(f); col += u_hot*ty.z*.18; }
  else { vec2 d = f - iTouch.xy; col = mix(col, u_ink, (1. - smoothstep(1., 2.5, abs(length(d) - 13.)))*iTouch.w); vec3 ty = typeAt(f); col = mix(col, u_ink, ty.x*.3); }
  o = vec4(col, 1.);
}`,
    bg: `
void mainImage(out vec4 o, in vec2 f){
  vec2 p = sect(f)*1.5;
  float a = atan(p.y, p.x), r = length(p) + 1e-4, k = PI/3.;
  a = abs(mod(a + iTime*.02, k) - .5*k);                   // the garden through a six-way kaleidoscope
  vec2 q = r*vec2(cos(a), sin(a));
  vec3 g = topTex(q*.7 + vec2(.05, .45 + iTime*.004));
  vec3 col = mix(u_ground, g, .6)*(.35 + .65*smoothstep(1.5, .2, r));
  col += screenLight(f)*.5;
  vec3 ty = titleAt(f); vec2 tu = titleUV(f);              // the headline, overgrown and eroding at its edges
  float n = fbm(tu*vec2(24., 6.) + vec2(0., iTime*.12));
  float inside = smoothstep(.46, .54, ty.y + .32*(n - .5));
  vec3 gt = botTex(tu*vec2(3., 1.2) + vec2(iTime*.01, 0.));
  vec3 fill = mix(u_ink*.9, gt*1.7 + u_hot*.08, smoothstep(.04, .22, dot(gt, vec3(.33))));
  col = mix(col, vec3(0.), smoothstep(.1, .4, ty.y)*(1. - inside)*.8);
  col = mix(col, u_cool, (1. - smoothstep(0., .08, abs(ty.y + .32*(n - .5) - .5)))*.7);
  col = mix(col, fill, inside);
  o = vec4(col, 1.);
}`
  },
  frame(h) {
    const s = h.state;
    if (s.t == null) Object.assign(s, { t: 0, seed: 1, seedB: 0, digit: 9 });
    s.t += h.dt;
    const d = 9 - Math.floor(s.t % 10);
    if (d !== s.digit) { s.digit = d; s.seedB = 1; if (d === 9) s.seed = 1; }
    s.seed = Math.max(0, s.seed - h.dt * 1.5); s.seedB = Math.max(0, s.seedB - h.dt * 4);
    const font = "'Climate Crisis', 'Arial Black', sans-serif";
    const over = h.typeField('over', { text: 'GAME\nOVER', size: 92, y: 192, weight: 400, font, lh: 1 });
    const cont = h.typeField('cont:' + d, { text: 'CONTINUE?\n' + d, size: 40, y: 192, weight: 400, font, lh: 1.6 });
    const rd = h.sim('rd', 'rd', 256, 430, { steps: 10, float: true, linear: true, uniforms: { u_seed: s.seed, u_seedB: s.seedB }, textures: { u_type: over, iChannel1: cont } });
    h.screen('top', 'bloom', { textures: { iChannel1: rd, u_type: over } });
    h.screen('bottom', 'bloom', { textures: { iChannel1: rd, u_type: cont } });
  }
},

/* ───────────────────────── 07 · draw the strands ───────────────────────── */
{
  id: 'strands', genre: 'Touch canvas, hyperreal', edge: 'tear', res: [256, 192], shell: [0.07, 0.03, 0.09],
  pal: { ground: '#050208', ink: '#FFFFFF', hot: '#FF00A8', cool: '#00FFD1' },
  title: 'DRAW THE\nSTRANDS', titleCss: "font-family:'Tilt Warp','Arial Black',sans-serif;font-size:clamp(54px,9vw,140px);line-height:.9",
  lede: 'The bottom screen is the place where reality is spun. Anything you draw becomes a strand: it creeps along its own length, bends in an invisible current and grows hairs. The top screen is an Apollonian gasket, circles packed inside circles forever, and your strands pull it out of shape. Behind the console the strands are as big as the page.',
  try: 'Draw on the bottom screen. Leave it alone and it draws by itself.',
  how: 'The strands are a 256 × 192 field of direction, density and hue. Each step moves every cell back along its own direction plus a curl-noise current, so a stroke travels along itself without spreading out. The gasket folds space into a cell, inverts it through the unit circle and repeats nine times; the strand field is added to the coordinates first. Dragging anywhere on this page also draws strands across the whole page.',
  glsl: {
    ink: RIFT_INK + `
uniform vec4 u_seg; uniform float u_down, u_hue;
void mainImage(out vec4 o, in vec2 f){
  vec2 sz = vec2(256., 192.);
  if (iFrame == 0) { o = vec4(.5, .5, 0., 0.); return; }
  vec4 s0 = texture(iChannel0, f/sz);
  vec2 c = curl(f/sz.y*2.5 + iTime*.04);
  vec2 back = f - ((s0.xy*2. - 1.)*.8 + c*.7);           // each strand creeps along itself; the current bends it
  vec4 s = texture(iChannel0, back/sz);
  vec2 v = normalize(mix(s.xy*2. - 1., normalize(c + 1e-4), .02) + 1e-4);
  float dens = s.z*.9965 - .0004, hue = s.w;
  float side = texture(iChannel0, (back + vec2(-v.y, v.x)*1.6*sin(iTime*2. + f.y*.07))/sz).z;   // and grows hairs
  dens = max(dens, side*.97*step(.93, vnoise(f*.1 + iTime*.7)));
  if (u_down > .5) {
    float d = sdSeg(f, u_seg.xy, u_seg.zw), m = exp(-d*d/2.2);
    dens = max(dens, m); v = normalize(mix(v, normalize(u_seg.zw - u_seg.xy + 1e-4), m) + 1e-4); hue = mix(hue, u_hue, m);
  }
  o = vec4(v*.5 + .5, clamp(dens, 0., 1.), hue);
}`,
    paint: RIFT_INK + `
void mainImage(out vec4 o, in vec2 f){
  vec2 uv = f/vec2(512., 384.), e = 1./vec2(256., 192.);
  vec4 s = texture(iChannel1, uv);
  vec2 v = s.xy*2. - 1.;
  float dx = texture(iChannel1, uv + vec2(e.x, 0.)).z - texture(iChannel1, uv - vec2(e.x, 0.)).z;
  float dy = texture(iChannel1, uv + vec2(0., e.y)).z - texture(iChannel1, uv - vec2(0., e.y)).z;
  vec3 n = normalize(vec3(-dx, -dy, .25));
  float fib = .5 + .5*sin(dot(f, vec2(-v.y, v.x))*1.4);   // fibres across each strand
  vec3 sheen = film(s.w + s.z*.6 + n.x*.4 + n.y*.25);       // thin-film colour that turns with the strand's slope
  float body = smoothstep(.04, .5, s.z);
  vec3 col = u_ground + u_cool*.03*fbm(f*.02 + iTime*.05);
  col = mix(col, sheen*(.55 + .45*fib), body);
  col += pow(max(dot(n, normalize(vec3(-.4, .5, .75))), 0.), 22.)*body*.9;
  col += u_ink*smoothstep(.85, 1., s.z)*.5;
  vec2 d = f - iTouch.xy; col = mix(col, u_ink, (1. - smoothstep(.8, 2., abs(length(d) - 7.)))*iTouch.w);
  o = vec4(col, 1.);
}`,
    gasket: RIFT_ATL + RIFT_INK + `
void mainImage(out vec4 o, in vec2 f){
  vec4 s = texture(iChannel1, f/vec2(512., 384.));          // the strands, seen through the hinge
  vec2 p = (f - vec2(256., 192.))/192.;
  p += (s.xy*2. - 1.)*s.z*.22;
  vec2 q = rot2(iTime*.03)*p*1.15 + vec2(.21*sin(iTime*.05), .1);
  float sc = 1., trap = 1e9;
  for (int i = 0; i < 9; i++) { q = -1. + 2.*fract(.5*q + .5); float r2 = dot(q, q), k = (1.18 + .04*sin(iTime*.3))/r2; q *= k; sc *= k; trap = min(trap, r2); }
  float d = .25*abs(q.y)/sc*192.;                          // distance to the nearest circle, in logical px
  vec3 col = u_ground + film(trap*.5 + iTime*.03)*.35*exp(-trap*1.2);
  col = mix(col, u_hot, exp(-d/1.1)*.9);
  col += u_cool*.25*exp(-d/9.);
  col += film(s.w)*smoothstep(.15, .9, s.z)*.4;
  o = vec4(col, 1.);
}`,
    bg: RIFT_INK + `
void mainImage(out vec4 o, in vec2 f){
  vec4 s = texture(iChannel1, clamp((f - u_band.xy)/u_band.zw, 0., 1.));   // what you drew on the bottom screen, as big as the page
  vec2 p = sect(f)*1.4;
  p += (s.xy*2. - 1.)*s.z*.14;
  p = rot(iTime*.02)*p + vec2(.3, .1);
  float sc = 1., trap = 1e9;
  for (int i = 0; i < 8; i++) { p = -1. + 2.*fract(.5*p + .5); float r2 = dot(p, p), k = (1.15 + .05*sin(iTime*.2))/r2; p *= k; sc *= k; trap = min(trap, r2); }
  float d = .25*abs(p.y)/sc*iStage.y/1.4/u_k;              // css px to the nearest circle
  vec3 col = u_ground + film(trap*.6 + iTime*.02)*.28*exp(-trap*.9);
  col = mix(col, u_hot, exp(-d/1.2)*.85);
  col += u_cool*.2*exp(-d/16.);
  col += film(s.w + s.z*.4)*(smoothstep(.1, .8, s.z)*.9 + s.z*.2);
  col += screenLight(f)*.5;
  vec3 ty = titleAt(f); vec2 tu = titleUV(f);              // the headline, an oil slick
  float th = fbm(tu*vec2(6., 2.) + iTime*.1) + ty.y*.8 + s.z*.5;
  vec3 oil = mix(film(th*1.7), vec3(.04), .2);
  col = mix(col, vec3(0.), smoothstep(.1, .4, ty.y)*(1. - ty.x));
  col = mix(col, oil, ty.x);
  o = vec4(col, 1.);
}`
  },
  frame(h) {
    const s = h.state, t = h.touch;
    if (s.idle == null) Object.assign(s, { idle: 3, last: null, ghost: 0 });
    let seg = null, down = 0;
    if (t.down) {
      s.idle = 0; const p = [t.x * 0.5, t.y * 0.5]; seg = [...(s.last || p), ...p]; s.last = p; down = 1;
    } else {
      s.idle += h.dt; s.last = null;
      if (s.idle > 2.5) {                                    // nobody is drawing: a ghost hand does
        s.ghost += h.dt; const g = s.ghost;
        const p = [128 + 100 * Math.sin(g * 1.3) * Math.cos(g * 0.37), 96 + 76 * Math.sin(g * 1.7 + 1)];
        if (Math.sin(g * 0.9) > -0.3) { seg = [...(s.gp || p), ...p]; down = 1; }
        s.gp = p;
      }
    }
    const ink = h.sim('ink', 'ink', 256, 192, { float: true, linear: true, uniforms: { u_seg: seg || [0, 0, 0, 0], u_down: down, u_hue: (h.time * 0.07) % 1 } });
    h.screen('top', 'gasket', { textures: { iChannel1: ink } });
    h.screen('bottom', 'paint', { textures: { iChannel1: ink } });
    h.bgTextures = { iChannel1: ink };
  }
}
);
