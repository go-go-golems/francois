/* phant-b.js — cartridges 1–3: save files that regress (Ubik), an inventory that fills with kipple, a HUD you fuse with. */
(function () {
'use strict';
const PH = window.PHANT = window.PHANT || { list: [], byId: {} };
const scene = (s) => { PH.list.push(s); PH.byId[s.id] = s; return s; };
const LH = 384;
const clamp = (x, a, b) => Math.max(a, Math.min(b, x));

/* ═══════════════════════ HALF-LIFE · save files in cold-pac (Ubik) ═══════════════════════ */
const SLOTS = [
  { who: 'JOE', game: 'KIPPLE', time: '41:07', md: '06-05', to: 'kipple' },
  { who: 'PAT', game: 'PINK BEAM', time: '12:58', md: '11-02', to: 'beam' },
  { who: 'GLEN', game: 'EMPATHY BOX', time: '03:33', md: '02-19', to: 'empathy' }
];
const ERAS = ['TRUE COLOUR', '64 COLOURS', '4 COLOURS', '1-BIT', 'PUNCH CARD', 'COLD'];
const eraOf = life => clamp(Math.floor((1 - life) * 5.8 - .4), 0, 5);
const MEMORY = `
// three remembered pictures, in true colour while they last
vec3 memory(int k, vec2 p, float t){
  vec3 col;
  if (k == 0) {                                                             // a street at sunset, 1959
    col = mix(vec3(1., .58, .32), vec3(.3, .2, .5), smoothstep(.12, .7, p.y));
    float sun = length(p - vec2(.68, .36)) - .075;
    col = mix(col, vec3(1., .93, .72), 1. - smoothstep(0., .005, sun)) + vec3(1., .55, .3)*.3*exp(-max(sun, 0.)*10.);
    float wire = abs(p.y - .5 + .05*cos((fract(p.x*2.5) - .5)*3.)); col = mix(col, vec3(.12, .08, .12), 1. - smoothstep(.0015, .004, wire));
    float hill = .25 + .025*sin(p.x*9. + 1.) + .02*fbm(vec2(p.x*8., 1.));
    float hid = floor(p.x*6.), hx = fract(p.x*6.) - .5, hh = .07 + .035*hash12(vec2(hid, 2.)), base = hill - .012;
    float wall = max(abs(hx) - .3, max(p.y - base - hh, base - .06 - p.y));
    float roof = max(abs(hx)*.22 + (p.y - base - hh) - .075, base + hh - p.y);
    if (p.y < hill) col = mix(vec3(.16, .28, .15), vec3(.05, .09, .05), clamp((hill - p.y)*4., 0., 1.));
    if (wall < 0.) { col = mix(vec3(.88, .78, .62), vec3(.62, .82, .86), hash12(vec2(hid, 5.)));
      vec2 w = vec2(abs(hx) - .14, p.y - base - hh*.5); col = mix(col, mix(vec3(1., .82, .4), vec3(.15, .15, .2), step(.6, hash12(vec2(hid, floor(t*.3))))), step(abs(w.x), .05)*step(abs(w.y), .014)); }
    if (roof < 0. && wall >= 0.) col = mix(vec3(.55, .22, .2), vec3(.3, .3, .38), hash12(vec2(hid, 7.)));
  } else if (k == 1) {                                                      // the moon, 1992: the Earth comes up over the domes
    col = vec3(.02, .02, .05) + vec3(step(.996, hash12(floor(p*300.))))*.8;
    float earth = length(p - vec2(.3, .56)) - .1;
    vec3 eC = mix(vec3(.15, .35, .8), vec3(.25, .6, .3), smoothstep(.45, .6, fbm(p*14. + vec2(t*.01, 0.))));
    eC = mix(eC, vec3(1.), smoothstep(.55, .7, fbm(p*9. - vec2(t*.02, 0.))));
    eC *= smoothstep(-.06, .05, (p.x - .3)*.7 + (p.y - .56)*.7 + .02);
    col = mix(col, eC, 1. - smoothstep(0., .004, earth)) + vec3(.2, .4, .9)*.25*exp(-max(earth, 0.)*40.);
    float ground = .2 + .04*fbm(vec2(p.x*4., 3.)) + .015*sin(p.x*30.);
    if (p.y < ground) { col = vec3(.5, .5, .52)*(.6 + .4*fbm(p*30.)); vec2 cp = fract(p*vec2(9., 14.)) - .5; col *= 1. - .3*smoothstep(.25, .2, length(cp))*step(.5, hash12(floor(p*vec2(9., 14.)))); }
    float dome = length((p - vec2(.72, ground))*vec2(1., 1.4)) - .12; float dome2 = length((p - vec2(.88, ground))*vec2(1., 1.4)) - .07;
    if (min(dome, dome2) < 0.) col = mix(vec3(.75, .8, .85), vec3(1., .85, .5), step(.7, fract((p.x - .6)*30.))*step(p.y, ground + .06));
  } else {                                                                  // a city at night with cars in the air
    col = mix(vec3(.05, .03, .12), vec3(.35, .12, .35), smoothstep(.75, .1, p.y));
    for (int i = 0; i < 3; i++) { float fi = float(i);
      float bw = .05 + .03*fi, id = floor((p.x + fi*.37)/bw), bh = .18 + .32*hash12(vec2(id, fi + 1.))*(1. - .3*fi);
      if (p.y < bh) { col = mix(vec3(.04, .03, .08), vec3(.1, .08, .16), fi*.4);
        vec2 wq = fract(vec2((p.x + fi*.37)/bw*4., p.y*60.)); col = mix(col, mix(vec3(1., .75, .35), vec3(.4, .9, 1.), hash12(vec2(id, 9.))), step(.55, wq.x)*step(.45, wq.y)*step(.55, hash12(vec2(floor((p.x + fi*.37)/bw*4.), floor(p.y*60.)))) *.9); } }
    for (int i = 0; i < 4; i++) { float fi = float(i); vec2 cp = vec2(fract(t*(.03 + .02*fi) + fi*.3)*1.2 - .1, .45 + .07*fi); float car = length((p - cp)*vec2(1., 3.)) - .012;
      col = mix(col, vec3(1., .3, .5), 1. - smoothstep(0., .004, car)); col += vec3(1., .3, .5)*.08*exp(-length(p - cp)*40.); }
  }
  return col;
}`;
scene({
  id: 'halflife', name: 'HALF-LIFE', status: 'FILE SELECT', element: 'Save files and file select', book: 'Ubik', cart: true,
  pal: { ground: '#10131A', ink: '#F4EFE6', hot: '#F2B134', cool: '#5B7CFA' },
  hint: ['Each save file runs down at its own speed, back through colour, four colours, one bit and the punch card.', 'Hold Y to spray Ubik on the chosen file. Messages from the other side show through the worn patches.', 'A loads the file into its cartridge, if it is still warm. B goes back.'],
  enter(S) { S.sel = 0; S.life = (PH.saveLife || [.97, .62, .36]).slice(); S.cold = 0; },
  update(S, G, I, dt) {
    if (I.press('UP')) S.sel = (S.sel + 2) % 3; if (I.press('DOWN')) S.sel = (S.sel + 1) % 3;
    if (I.tap) for (let i = 0; i < 3; i++) { const y = 50 + i * 98; if (I.tap[1] >= y && I.tap[1] <= y + 88) { if (i === S.sel) S.load = true; S.sel = i; } }
    const rate = [.006, .011, .017];
    for (let i = 0; i < 3; i++) S.life[i] = Math.max(0, S.life[i] - dt * rate[i]);
    S.spray = I.held('Y') ? Math.min(1, (S.spray || 0) + dt * 5) : Math.max(0, (S.spray || 0) - dt * 3);
    if (I.held('Y')) S.life[S.sel] = Math.min(1, S.life[S.sel] + dt * .45);
    PH.saveLife = S.life; S.cold = Math.max(0, S.cold - dt);
    if (I.press('A') || S.load) { S.load = false; if (S.life[S.sel] < .1) S.cold = 2; else G.go(SLOTS[S.sel].to, 'regress'); }
    if (I.press('B')) G.go('home', 'regress');
  },
  focus: (S) => [256, LH - (50 + S.sel * 98 + 44)],
  sig: (S, G, s) => s ? S.life.map(l => `${eraOf(l)}${Math.round(1992 - (1 - l) * 53)}`).join('|') + S.sel + (S.cold > 0) : `${S.sel}|${Math.round(1992 - (1 - S.life[S.sel]) * 53)}`,
  ui: {
    top(U, S) {
      const s = SLOTS[S.sel], yr = Math.round(1992 - (1 - S.life[S.sel]) * 53);
      U.text(`${s.who}, ${yr}`, 24, 352, { f: 'jac', size: 44 });
      const msgs = [['THIS SAVE IS STILL WARM', 60, 110, -.08], ['I AM ON THE OTHER SIDE OF THE FILE', 150, 200, .05], ['SPRAY BEFORE IT IS TOO LATE', 230, 290, -.03], ['WE ARE ALL IN HERE WITH YOU', 40, 260, .04]];
      msgs.forEach(([t, x, y, r], i) => { if (i !== 3 || S.sel === 2) U.text(t, x, y, { f: 'cou', w: 700, size: 22, ch: 'b', rot: r }); });
    },
    bot(U, S) {
      U.text('FILE SELECT', 16, 34, { size: 16 }); U.text('HOLD Y: UBIK', 496, 34, { size: 8, align: 'right', track: 2, a: .8 });
      SLOTS.forEach((s, i) => {
        const y = 50 + i * 98, l = S.life[i], yr = Math.round(1992 - (1 - l) * 53);
        U.rect(14, y, 484, 88, { ch: 'g', r: 5 });
        U.text(`FILE ${i + 1}  ${s.who}`, 150, y + 28, { size: 16 });
        U.text(`${s.game}   ${s.time}`, 150, y + 52, { f: 'vt', size: 22 });
        U.text(`SAVED ${yr}-${s.md}   ${ERAS[eraOf(l)]}`, 150, y + 76, { f: 'vt', size: 22, a: .85 });
      });
      U.text(S.cold > 0 ? 'THIS FILE HAS GONE COLD. SPRAY IT FIRST.' : 'A LOAD    Y SPRAY UBIK    B BACK', 256, 368, { f: 'vt', size: 22, align: 'center' });
    }
  },
  uni: (S) => ({ u_sel: S.sel, u_p0: [0, 0, S.spray || 0, 0], u_p1: [...S.life, 0] }),
  common: MEMORY,
  glsl: {
    top: `
void mainImage(out vec4 o, in vec2 f){
  int k = int(u_sel + .5); float life = k == 0 ? u_p1.x : k == 1 ? u_p1.y : u_p1.z;
  float e = eraAt(f + float(k)*97., life);
  vec2 s = eraSrc(f, e);
  vec3 col = eraCol(memory(k, s/512., u_t), f, e, ipx(f));
  col += u_hot*.5*(1. - smoothstep(0., .04, fract(e)))*step(.5, e);       // the seams where one year gives way to an older one
  vec3 U = ui(f);
  col = mix(col, u_hot, smoothstep(.3, .6, U.b)*smoothstep(2.4, 2.9, e));  // messages from the other side, only where reality has worn through
  col = mix(col, col*.25, smoothstep(.05, .4, uiL(f, 2.).r));
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  float sp = u_p0.z;
  if (sp > .01) {                                                          // Ubik, sprayed from the right
    vec2 dir = normalize(vec2(-1., .45)), q = f - vec2(512., 70.);
    float along = dot(q, dir), across = dot(q, vec2(-dir.y, dir.x));
    float cone = step(0., along)*smoothstep(along*.35 + 6., along*.18, abs(across))*smoothstep(420., 60., along);
    float grain = step(.8, hash12(floor(f/2.) + floor(u_now*30.)*vec2(3., 7.)));
    col = mix(col, vec3(1., .86, .45), cone*grain*sp) + vec3(1., .8, .4)*cone*sp*.15;
  }
  o = vec4(col, 1.);
}`,
    bot: `
void mainImage(out vec4 o, in vec2 f){
  float yd = 384. - f.y; int j = -1;
  for (int i = 0; i < 3; i++) if (yd >= 50. + float(i)*98. && yd < 138. + float(i)*98.) j = i;
  float life = j == 0 ? u_p1.x : j == 1 ? u_p1.y : j == 2 ? u_p1.z : 1.;
  float e = j >= 0 ? eraAt(f*1.3 + float(j)*53., life) : 0.;
  vec2 s = eraSrc(f, e);
  vec3 U = ui(s), col = u_ground;
  col = mix(col, mixOk(u_ground, u_cool, .2), smoothstep(.3, .6, U.g));
  if (j >= 0) { vec2 tp = (s - vec2(26., 384. - (50. + float(j)*98. + 80.)))/vec2(110., 72.);
    if (all(greaterThan(tp, vec2(0.))) && all(lessThan(tp, vec2(1.)))) col = memory(j, vec2(tp.x, tp.y*.75), u_t); }
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  if (j >= 0) col = eraCol(col, f, e, ipx(f));
  float sel = floor(u_sel + .5);
  col = mix(col, u_hot, aa(abs(sdBox(f - vec2(256., 384. - (50. + sel*98. + 44.)), vec2(243., 45.))) - 1.)*(.7 + .3*sin(u_t*5.)));
  o = vec4(col, 1.);
}`
  }
});

/* ═══════════════════════ KIPPLE · the inventory (Do Androids Dream of Electric Sheep?) ═══════════════════════ */
const ITEMS = [
  ['ELECTRIC SHEEP', 'Grazes on three volts. The wool has to be replaced every spring.', 4],
  ['MOOD ORGAN', 'Dial a feeling. Someone has left it set to wanting to watch television.', 61],
  ['EMPATHY BOX', 'Two handles. While you hold them you are not alone.', 88],
  ['OWL', 'Listed as extinct. No price is given.', 50],
  ['TOAD', 'Very rare. Check its belly for a little control panel.', 12],
  ['SPIDER', 'Somebody has been counting its legs.', 70],
  ['LASER TUBE', 'Standard issue. Do not point it at anyone you are unsure about.', 99],
  ['KIPPLE', 'Junk mail, empty matchbooks, gum wrappers. It reproduces when nobody is looking.', 100]
];
const START_INV = [0, 1, -1, 3, 6, -1, -1, 2, 4, -1, 7, 5, 6, -1, 0, -1, -1, 1, -1, 5, -1, 4, -1, 3];
const ITEM_GLSL = `
uniform vec4 u_cells[6], u_kip[6];
float cellV(vec4 a[6], int i){ return a[i/4][i - 4*(i/4)]; }
// the things you own, drawn in units where the slot is -1..1; ppu: logical px per unit
vec4 item(int id, vec2 p, float t, float ppu){
  float d = 1e9; vec3 c = u_ink; float px = 1./(ppu*iResolution.z);
  if (id == 0) {
    for (int k = 0; k < 7; k++) { float fk = float(k)*.9; d = smin(d, length(p - vec2(-.1 + cos(fk)*.36, .05 + sin(fk)*.2)) - .24, .06); }
    float wool = d, head = length((p - vec2(.55, .24))*vec2(1., 1.25)) - .17, legs = min(sdBox(p - vec2(-.32, -.42), vec2(.05, .17)), sdBox(p - vec2(.18, -.42), vec2(.05, .17)));
    d = min(d, min(head, legs)); c = wool < 0. ? u_ink*(.85 + .15*sin(length(p)*40.)) : mixOk(u_ground, u_ink, .35);
    c = mix(c, u_hot, clamp(.5 - (sdSeg(p, vec2(-.05, .12), vec2(.06, -.04)) - .04)/px, 0., 1.)*step(wool, 0.));
  } else if (id == 1) {
    float r = length(p); d = r - .78; c = mixOk(u_ground, u_cool, .55);
    c = mix(c, u_ink, step(.58, r)*step(r, .72)*step(.8, fract(atan(p.y, p.x)/6.2832*12.)));
    c = mix(c, u_hot, clamp(.5 - (sdSeg(p, vec2(0.), .55*vec2(cos(t*.6 + 2.), sin(t*.6 + 2.))) - .05)/px, 0., 1.));
    c = mix(c, u_ink, step(length(p), .1));
  } else if (id == 2) {
    float box = sdBox(p - vec2(0., -.1), vec2(.5, .42)) - .04, scr = sdBox(p - vec2(0., -.05), vec2(.36, .27)), hl = sdBox(vec2(abs(p.x) - .64, p.y + .05), vec2(.07, .2));
    d = min(box, hl); c = mix(mixOk(u_cool, u_ground, .3), u_ink, step(hl, 0.));
    c = mix(c, mixOk(u_hot, u_ink, .3 + .3*sin(t*2.)), step(scr, 0.));
  } else if (id == 3) {
    float body = length((p - vec2(0., -.1))*vec2(1.25, 1.)) - .62, ears = min(sdSeg(p, vec2(-.38, .42), vec2(-.5, .78)), sdSeg(p, vec2(.38, .42), vec2(.5, .78))) - .06;
    d = min(body, ears); c = mixOk(u_hot, u_ground, .45);
    vec2 e = vec2(abs(p.x) - .24, p.y - .12); float er = length(e);
    float spiral = .5 + .5*sin(atan(e.y, e.x)*2. + er*40. - t*3.);
    c = mix(c, mix(u_ink, u_ground, spiral*step(.06, er)), step(er, .2));
  } else if (id == 4) {
    float body = length((p - vec2(0., -.2))*vec2(1., 1.5)) - .7, eyes = length(vec2(abs(p.x) - .32, p.y - .25)) - .14;
    d = min(body, eyes); c = mixOk(u_cool, u_ground, .35);
    c = mix(c, u_ink, step(eyes, 0.)); c = mix(c, u_ground, step(length(vec2(abs(p.x) - .32, p.y - .25)), .06));
    float panel = sdBox(p - vec2(0., -.35), vec2(.22, .12)); c = mix(c, mixOk(u_hot, u_ink, .4), step(panel, 0.)*(.6 + .4*step(.5, fract(t*2.))));
  } else if (id == 5) {
    float body = min(length(p - vec2(0., -.05)) - .22, length(p - vec2(0., .25)) - .14);
    float legs = 1e9; for (int k = 0; k < 4; k++) { float fk = float(k); float a = .5 + fk*.35; vec2 knee = vec2(.45, -.1 + .25*sin(a*2. + t*3. + fk));
      legs = min(legs, min(sdSeg(vec2(abs(p.x), p.y), vec2(.15, .05 - fk*.08), knee), sdSeg(vec2(abs(p.x), p.y), knee, vec2(.75, -.55 + fk*.12)))); }
    d = min(body, legs - .035); c = mix(u_ink, mixOk(u_ground, u_ink, .6), step(0., body));
  } else if (id == 6) {
    float tube = sdBox(rot2(.6)*p, vec2(.62, .1)) - .06, beam = abs(dot(p - vec2(.42, .32), normalize(vec2(-.68, .57))))*.0 + sdSeg(p, vec2(.45, .38), vec2(.95, .8)) - .03;
    d = min(tube, beam); c = mix(mixOk(u_ground, u_ink, .6), mixOk(u_hot, u_ink, .3 + .3*sin(t*12.)), step(beam, 0.));
  } else {
    vec2 g = p*3.; vec2 id = floor(g); float best = 9., b2 = 9.; vec2 bid = id;
    for (int y = -1; y <= 1; y++) for (int x = -1; x <= 1; x++) { vec2 n = id + vec2(float(x), float(y)); vec2 cp = n + hash22(n); float dd = length(g - cp); if (dd < best) { b2 = best; best = dd; bid = n; } else b2 = min(b2, dd); }
    d = length(p*vec2(.9, 1.15)) - .8 + .12*fbm(p*4.);
    float hs = hash12(bid);
    c = mixOk(mixOk(u_ground, u_hot, .35 + .4*hs), u_ink, step(.75, hs)*.5)*(.6 + .4*smoothstep(0., .15, b2 - best));
  }
  return vec4(c, clamp(.5 - d/px, 0., 1.));
}`;
scene({
  id: 'kipple', name: 'KIPPLE', status: 'INVENTORY', element: 'Inventory grid', book: 'Do Androids Dream of Electric Sheep?', cart: true,
  pal: { ground: '#18150F', ink: '#E6DAC2', hot: '#C9D33A', cool: '#8F74F2' },
  hint: ['Kipple grows into empty slots and spreads to its neighbours. Full slots go to kipple too, in the end.', 'Touch a slot to examine it, touch it again (or press A) to sort it. Dragging across the grid sweeps.', 'When twenty slots are kipple, kipple has won.'],
  enter(S) { S.sel = 7; S.inv = (PH.inv || START_INV).slice(); S.kip = S.inv.map(v => v === 7 ? 1 : Math.random() * .3); S.pulse = 0; S.fresh = true; },
  update(S, G, I, dt) {
    let sel = S.sel; const c = sel % 6, r = Math.floor(sel / 6);
    if (I.press('LEFT')) sel = r * 6 + (c + 5) % 6; if (I.press('RIGHT')) sel = r * 6 + (c + 1) % 6;
    if (I.press('UP')) sel = ((r + 3) % 4) * 6 + c; if (I.press('DOWN')) sel = ((r + 1) % 4) * 6 + c;
    let act = I.press('A');
    if (I.tap) { const cx = Math.floor((I.tap[0] - 44) / 72), ry = Math.floor((I.tap[1] - 56) / 72); if (cx >= 0 && cx < 6 && ry >= 0 && ry < 4) { const i = ry * 6 + cx; if (i === sel) act = true; sel = i; } }
    if (I.touch.down) { const cx = Math.floor((I.touch.x - 44) / 72), ry = Math.floor((LH - I.touch.y - 56) / 72); if (cx >= 0 && cx < 6 && ry >= 0 && ry < 4) S.kip[ry * 6 + cx] = Math.max(0, S.kip[ry * 6 + cx] - dt * .8); }
    if (sel !== S.sel) S.pulse = 1; S.sel = sel; S.pulse = Math.max(0, S.pulse - dt * 2);
    if (act) { if (S.inv[sel] === 7) S.inv[sel] = -1; S.kip[sel] = 0; S.pulse = 1; }
    const next = S.kip.slice();
    for (let i = 0; i < 24; i++) {
      if (S.inv[i] === 7) { next[i] = 1; continue; }
      const cc = i % 6, rr = Math.floor(i / 6); let n = 0;
      for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) { const x = cc + dx, y = rr + dy; if (x >= 0 && x < 6 && y >= 0 && y < 4 && S.inv[y * 6 + x] === 7) n++; }
      next[i] = Math.min(1, S.kip[i] + dt * (.004 + .045 * n + (S.inv[i] < 0 ? .012 : 0)));
      if (next[i] >= 1) S.inv[i] = 7;
    }
    S.kip = next; PH.inv = S.inv;
    const n7 = S.inv.filter(v => v === 7).length; S.frac = n7 / 24;
    if (n7 >= 20) { PH.inv = null; G.over('KIPPLE DRIVES OUT NONKIPPLE'); }
    if (I.press('B')) G.go('home', 'regress');
  },
  focus: (S) => [44 + (S.sel % 6) * 72 + 32, LH - (56 + Math.floor(S.sel / 6) * 72 + 32)],
  sig: (S, G, s) => s ? `${S.inv.filter(v => v === 7).length}` : `${S.inv[S.sel]}|${S.sel}`,
  ui: {
    top(U, S) {
      const id = S.inv[S.sel], it = id < 0 ? ['EMPTY SLOT', 'Nothing is here yet. Something will be.', 0] : ITEMS[id];
      U.text(it[0], 300, 92, { f: 'jac', size: id === 0 ? 34 : 40 });
      U.wrap(it[1], 300, 124, 196, { f: 'vt', size: 22, lh: 21 });
      U.text('VOIGHT-KAMPFF', 300, 296, { size: 8, track: 2, a: .8 }); U.text(`AUTHENTIC ${it[2]}%`, 300, 348, { size: 16 });
    },
    bot(U, S) {
      U.text('INVENTORY', 44, 40, { size: 16 }); U.text(`KIPPLE ${S.inv.filter(v => v === 7).length}/24`, 468, 40, { size: 16, align: 'right', ch: 'rb' });
      U.text('TOUCH TWICE OR A TO SORT    DRAG TO SWEEP', 256, 372, { size: 8, align: 'center', a: .7, track: 1 });
    }
  },
  tex(h, S) {
    if (S.fresh) { const R = h.sims.get('kipple.rd'); if (R) R.steps = 0; S.fresh = false; }
    const rd = h.sim('kipple.rd', 'kipple.sim', 128, 96, { steps: 3, float: true, linear: true, uniforms: { u_kip: new Float32Array(S.kip) } });
    return { u_mem: rd };
  },
  uni: (S) => ({ u_sel: S.sel, u_cells: new Float32Array(S.inv), u_kip: new Float32Array(S.kip), u_p0: [S.inv[S.sel], S.inv[S.sel] < 0 ? 0 : ITEMS[S.inv[S.sel]][2] / 100, S.pulse, S.frac || 0] }),
  common: ITEM_GLSL,
  glsl: {
    sim: `
uniform vec4 u_kip[6];
float kipAt(vec2 L){
  float col = floor((L.x - 44.)/72.), row = floor(((384. - L.y) - 56.)/72.);
  if (col < 0. || col > 5. || row < 0. || row > 3.) return .3;
  vec2 lc = vec2(L.x - 44. - col*72., (384. - L.y) - 56. - row*72.);
  if (lc.x > 64. || lc.y > 64.) return .45;                                  // kipple likes the cracks between slots
  int i = int(row)*6 + int(col); return u_kip[i/4][i - 4*(i/4)];
}
void mainImage(out vec4 o, in vec2 f){
  vec2 L = f*4.;
  vec4 c = prev(f); if (iFrame == 0) c = vec4(1., 0., 0., 0.);
  vec4 n = prev(f + vec2(0., 1.)) + prev(f - vec2(0., 1.)) + prev(f + vec2(1., 0.)) + prev(f - vec2(1., 0.));
  vec4 g = prev(f + vec2(1.)) + prev(f - vec2(1.)) + prev(f + vec2(1., -1.)) + prev(f + vec2(-1., 1.));
  vec2 lap = iFrame == 0 ? vec2(0.) : -c.xy + .2*n.xy + .05*g.xy;
  float kv = kipAt(L);
  float F = .0545, k = mix(.07, .062, smoothstep(.05, .85, kv));
  float A = c.x, B = c.y, r = A*B*B;
  A += lap.x - r + F*(1. - A);
  B += .5*lap.y + r - (k + F)*B;
  B = max(B, step(.9975, hash12(f + floor(iTime*3.)*vec2(7., 3.)))*smoothstep(.15, .6, kv));   // kipple seeds itself
  vec2 d = L - iTouch.xy; B *= 1. - iTouch.z*step(dot(d, d), 700.);                              // the stylus sweeps it away
  o = vec4(clamp(A, 0., 1.), clamp(B, 0., 1.), 0., 1.);
}`,
    top: `
float foam(vec2 p){ float trap = 9.; for (int i = 0; i < 6; i++) { p = -1. + 2.*fract(.5*p + .5); float r2 = dot(p, p); p *= 1.2/r2; trap = min(trap, r2); } return trap; }
void mainImage(out vec4 o, in vec2 f){
  int id = int(u_p0.x + (u_p0.x < 0. ? -.5 : .5));
  vec3 col = u_ground*(.8 + .2*f.y/384.);
  vec2 ctr = vec2(150., 190.), p = (f - ctr)/105.;
  col += u_cool*.08*exp(-length(p)*1.5)*(.6 + .4*sin(length(p)*20. - u_t*2.));
  col = mix(col, u_ground*.6, (1. - smoothstep(0., 3., abs(length(f - ctr) - 140.)))*.0);
  if (id >= 0) {
    vec4 it = item(id, p/(1. + .06*u_p0.z), u_t, 105.);
    float fm = foam(rot2(u_t*.05)*p*1.6 + float(id)*1.7);              // looked at closely, everything you own is foam
    vec3 inner = it.rgb*(.55 + .6*smoothstep(.0, .5, fm)) + mixOk(u_hot, u_cool, fract(fm*2. + u_t*.1))*.25*exp(-fm*3.);
    col = mix(col, inner, it.a);
    col += mixOk(u_hot, u_ink, .5)*exp(-max(length(p) - .9, 0.)*5.)*.12*(1. - it.a);
  } else { float n = fbm(f*.02 + u_t*.2); col = mix(col, u_ink*.2, smoothstep(.55, .8, n)*smoothstep(150., 40., length(f - ctr))); }
  vec3 U = ui(f);
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  vec2 b = f - vec2(300., 384. - 316.);                                  // the authenticity bar, with its scan line
  if (b.x > 0. && b.x < 196. && b.y > -6. && b.y < 6.) { float fill = step(b.x/196., u_p0.y); col = mix(u_ground*1.4, mixOk(u_hot, u_cool, b.x/196.), fill);
    col = mix(col, u_ink, (1. - smoothstep(0., 2., abs(b.x - fract(u_t*.5)*196.)))*.8); }
  o = vec4(col, 1.);
}`,
    bot: `
void mainImage(out vec4 o, in vec2 f){
  vec3 col = u_ground, U = ui(f);
  float B = texture(u_mem, f/LS).y;
  float cx = floor((f.x - 44.)/72.), ry = floor(((384. - f.y) - 56.)/72.);
  vec2 lc = vec2(f.x - 44. - cx*72., (384. - f.y) - 56. - ry*72.);
  if (cx >= 0. && cx < 6. && ry >= 0. && ry < 4. && lc.x < 64. && lc.y < 64.) {
    int i = int(ry)*6 + int(cx);
    float id = cellV(u_cells, i), kv = cellV(u_kip, i);
    col = mixOk(u_ground, u_cool, .13)*(1. - .35*kv);
    col = mix(col, u_ground*.7, (1. - smoothstep(0., 2., min(min(lc.x, lc.y), min(64. - lc.x, 64. - lc.y))))*.6);
    if (id > -.5) { vec4 it = item(int(id + .5), vec2(lc.x - 32., 32. - lc.y)/27., u_t + float(i)*1.3, 27.); col = mix(col, it.rgb*(1. - .5*kv*step(id, 6.5)), it.a); }
    if (i == int(u_sel + .5)) col = mix(col, u_hot, (1. - smoothstep(0., 2., abs(sdBox(lc - 32., vec2(31.))) - .5 - 2.*u_p0.z)));
  }
  float kb = smoothstep(.12, .3, B);                                        // kipple, everywhere it has got to
  vec3 kc = mixOk(mixOk(u_ground, u_hot, .5), u_ink, smoothstep(.28, .5, B)*.45)*(.7 + .3*vnoise(f*.5));
  col = mix(col, kc, kb*.92);
  col = mix(col, u_ink, smoothstep(.3, .6, U.r));
  col = mix(col, u_hot, smoothstep(.3, .6, U.b));
  vec2 d = f - iTouch.xy; col = mix(col, u_ink, (1. - smoothstep(1., 2.5, abs(length(d) - 26.)))*iTouch.w*.8);
  o = vec4(col, 1.);
}`
  }
});

/* ═══════════════════════ EMPATHY BOX · the HUD (Mercerism) ═══════════════════════ */
scene({
  id: 'empathy', name: 'EMPATHY BOX', status: 'HUD', element: 'HUD: hearts, health bar, score', book: 'Do Androids Dream of Electric Sheep?', cart: true,
  pal: { ground: '#140E0C', ink: '#F3E4D2', hot: '#FF5A36', cool: '#E9B44C' },
  hint: ['The bottom screen is a game’s HUD: hearts, a health tree, a score.', 'Hold A, or touch a handle at the edge, to fuse. The hinge goes away and you climb with everyone else.', 'While fused, the stones thrown at the climber hit your hearts. Let go to heal.'],
  enter(S) { S.hp = PH.hp ?? 12; S.fused = 0; S.others = 4103226; S.score = 0; S.rocks = []; S.next = 1; S.hit = 9; S.climb = 0; },
  update(S, G, I, dt) {
    const t = I.touch, grip = I.held('A') || (t.down && (t.x < 60 || t.x > 452));
    S.fused += ((grip ? 1 : 0) - S.fused) * (1 - Math.exp(-dt * 3));
    S.hit += dt;
    if (S.fused > .8) {
      S.others += Math.floor(dt * (20000 + Math.random() * 60000)); S.score += dt * 10; S.climb += dt * .012;
      S.next -= dt; if (S.next <= 0) { S.rocks.push({ t0: S.t, side: Math.random() < .5 ? -1 : 1 }); S.next = 1.1 + Math.random() * 1.6; }
    } else { S.hp = Math.min(12, S.hp + dt * .3); S.others = Math.max(4103226, S.others - Math.floor(dt * 30000)); }
    S.rocks = S.rocks.filter(r => { if (S.t - r.t0 >= .9) { S.hp -= 1; S.hit = 0; const fx = .42 + .1 * Math.sin(S.climb * 6.28); G.splat(fx * 512, (.14 + .62 * fx) * 860 + 40, .7); return false; } return true; });
    PH.hp = S.hp;
    if (S.hp <= 0) { PH.hp = 12; G.over('THE CLIMB IS OVER'); }
    if (I.press('B')) G.go('home', 'regress');
  },
  focus: (S) => S.fused > .5 ? [256, 500] : [150, 200],
  sig: (S, G, s) => s ? `${Math.floor(S.others / 997)}|${Math.floor(S.score)}|${Math.ceil(S.hp)}` : '',
  ui: {
    top(U) { U.text('MERCER IS CLIMBING', 24, 358, { size: 16, ch: 'g', a: .9 }); },
    bot(U, S) {
      U.text(`HP ${Math.ceil(S.hp)}/12`, 300, 60, { size: 16 });
      U.text('OTHERS FUSED', 300, 120, { size: 8, track: 2, a: .8 }); U.text(S.others.toLocaleString('en-US'), 300, 158, { size: 32, ch: 'g' });
      U.text('EMPATHY', 300, 206, { size: 8, track: 2, a: .8 }); U.text(String(Math.floor(S.score)).padStart(6, '0'), 300, 244, { size: 32, ch: 'g' });
      U.rect(0, 112, 26, 170, { ch: 'b', r: 6 }); U.rect(486, 112, 26, 170, { ch: 'b', r: 6 });
      U.text('HOLD A OR A HANDLE TO FUSE', 256, 362, { f: 'vt', size: 22, align: 'center' });
    }
  },
  uni: (S) => { const r = new Float32Array(16); S.rocks.slice(0, 4).forEach((k, i) => r.set([S.t - k.t0, k.side, 1, 0], i * 4)); return { u_p0: [S.fused, S.hp, S.hit, S.climb], u_p2: [0, 0, 0, 0], u_rocks: r }; },
  common: `
uniform vec4 u_rocks[4];
float ground(float x){ return .14 + .62*x + .05*(fbm(vec2(x*5., 1.)) - .5) + .012*sin(x*40.); }
float sdHeart(vec2 p){ p.x = abs(p.x); if (p.y + p.x > 1.) return sqrt(dot(p - vec2(.25, .75), p - vec2(.25, .75))) - .35355;
  return sqrt(min(dot(p - vec2(0., 1.), p - vec2(0., 1.)), dot(p - .5*max(p.x + p.y, 0.), p - .5*max(p.x + p.y, 0.))))*sign(p.x - p.y); }
// the hill: q is 0..1 across, 0..1 up; H is the height in logical px that q.y spans
vec3 climb(vec2 q, float H, float t, float fused){
  float x = q.x;
  vec3 col = mixOk(mixOk(u_cool, u_hot, .35), u_ground, smoothstep(.2, 1., q.y));
  vec2 sp = (q - vec2(.8, .8))*vec2(1., H/512.);
  float sun = length(sp) - .13;
  col = mix(col, mixOk(u_cool, u_ink, .3), (1. - smoothstep(0., 2./H, sun))*step(.35, fract(q.y*H/9. + t*.3) + smoothstep(.85, .7, q.y)));
  float g = ground(x);
  float fx = .42 + .1*sin(u_p0.w*6.2832), fy = ground(fx);
  float ppx = 1./H;
  if (q.y < g) col = mix(mixOk(u_ground, u_cool, .25), u_ground*.6, clamp((g - q.y)*3., 0., 1.)) + u_cool*.05*fbm(q*vec2(40., 120.));
  float crowd = step(.6, hash12(vec2(floor(x*180.), 3.)))*step(q.y, g + .018 + .01*hash12(vec2(floor(x*180.), 4.)))*step(g, q.y);
  col = mix(col, u_ground*.4, crowd*fused*step(abs(x - fx), .6)*.85);
  vec2 fp = (q - vec2(fx, fy))*vec2(512./H, 1.)/.07;                       // the old man, his robe and his staff
  float robe = max(sdBox(fp - vec2(0., .38), vec2(.17 - .1*fp.y, .4)), -fp.y), head = length(fp - vec2(.02, .92)) - .12, staff = sdSeg(fp, vec2(.3, 0.), vec2(.42, 1.1)) - .025;
  float fig = min(min(robe, head), staff);
  col = mix(col, u_ground*.35, clamp(.5 - fig*.07*H*iResolution.z, 0., 1.));
  for (int i = 0; i < 4; i++) { vec4 r = u_rocks[i]; if (r.z < .5) continue;
    float s = clamp(r.x/.9, 0., 1.);
    vec2 a = vec2(r.y < 0. ? -.05 : 1.05, .9 + .05*r.y), b = vec2(fx, fy + .06);
    vec2 rp = mix(a, b, s) + vec2(0., .22*sin(PI*s));
    float rd = length((q - rp)*vec2(512./H, 1.)) - .012*(1. + .3*sin(atan(q.y - rp.y, q.x - rp.x)*5.));
    col = mix(col, mixOk(u_ground, u_hot, .4), clamp(.5 - rd*H*iResolution.z, 0., 1.)); }
  return col;
}`,
  glsl: {
    top: `
void mainImage(out vec4 o, in vec2 f){
  float fu = smoothstep(0., 1., u_p0.x);
  vec2 pos = mix(f, f + vec2(0., 384. + DS_GAP), fu); float H = mix(384., CS.y, fu);
  vec3 col = climb(pos/vec2(512., H), H, u_t, fu);
  col = mix(col, u_hot, exp(-u_p0.z*3.)*.45*smoothstep(150., 380., length(f - vec2(256., 192.))));
  vec3 U = ui(f); col = mix(col, u_ink, smoothstep(.3, .6, U.g)*(1. - fu));
  o = vec4(col, 1.);
}`,
    bot: `
float tree(vec2 p, float alive, out float lv){
  float d = 1e9, s = 1.; lv = 0.; const float L = .3, R = .72;
  for (int i = 0; i < 8; i++) {
    float di = (sdSeg(p, vec2(0.), vec2(0., L)) - .045)*s;
    if (di < d) { d = di; lv = float(i); }
    p.y -= L; p.x = abs(p.x); p = rot2(.5 + .08*sin(u_t + float(i)))*p; p /= R; s *= R;
  }
  return d;
}
void mainImage(out vec4 o, in vec2 f){
  float fu = smoothstep(0., 1., u_p0.x), hp = u_p0.y;
  vec3 hud = u_ground*(.9 + .1*f.y/384.);
  vec3 U = ui(f);
  for (int i = 0; i < 6; i++) {                                              // hearts, beating faster as they empty
    vec2 c = vec2(40. + float(i)*44., 384. - 62.); float fi = float(i);
    float beat = 1. + .08*pow(.5 + .5*sin(u_t*(6. + (12. - hp)*.8)), 6.);
    vec2 p = (f - c)/(17.*beat) + vec2(0., .5);
    float d = sdHeart(p)*17.;
    float full = clamp(hp*.5 - fi, 0., 1.);
    vec2 z = (p - vec2(0., .5))*1.4; vec2 jc = vec2(-.4, .6) + .05*vec2(cos(u_t), sin(u_t*1.3)); float tr = 9.;
    for (int k = 0; k < 10; k++) { z = vec2(z.x*z.x - z.y*z.y, 2.*z.x*z.y) + jc; tr = min(tr, length(z)); }
    vec3 hc = mixOk(u_hot, u_ink, exp(-tr*4.)*.8);
    float shown = full >= 1. ? 1. : step(p.x, 0.)*step(.5, full);
    hud = mix(hud, mix(u_ground*1.6, hc, shown), aa(d));
    hud = mix(hud, u_ink, aa(abs(d) - .8)*.8);
    if (full <= 0.) hud = mix(hud, u_ground, aa(abs(p.x - .1*sin(p.y*14.))*17. - .7)*step(d, 0.));
  }
  float lv; vec2 tp = (f - vec2(150., 40.))/250.;
  float td = tree(tp, 0., lv)*250.;
  float alive = hp/12.*8.;
  vec3 tc = lv < alive ? mixOk(u_cool, u_ink, lv/8.) : mixOk(u_ground, u_ink, .18);
  hud = mix(hud, tc, aa(td));
  if (lv >= alive - 1. && lv < alive) hud += u_hot*.6*exp(-max(td, 0.)/2.)*(.5 + .5*sin(u_t*4.));
  vec2 dg = mod(f*vec2(4., 3.), vec2(512., 384.));                           // the numbers are made of numbers
  float sub = smoothstep(.3, .6, textureLod(u_ui, dg/LS, UILOD() + 1.).g);
  hud = mix(hud, mix(u_hot, u_ink, sub), smoothstep(.3, .6, U.g));
  hud = mix(hud, u_ink, smoothstep(.3, .6, U.r));
  hud = mix(hud, mixOk(u_cool, u_hot, fu), smoothstep(.3, .6, U.b)*(.5 + .5*fu));
  vec3 hill = climb(f/vec2(512., CS.y), CS.y, u_t, fu);
  vec3 col = mix(hud, mix(hill, hud, .22), fu);
  col = mix(col, u_hot, exp(-u_p0.z*3.)*.35*smoothstep(150., 380., length(f - vec2(256., 192.))));
  o = vec4(col, 1.);
}`
  }
});
})();
