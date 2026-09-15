import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js';
import { RoundedBoxGeometry } from 'three/addons/geometries/RoundedBoxGeometry.js';

const OPEN_ANGLE = -100 * (Math.PI / 180);
const CLOSED_ANGLE = 0;
const LID_MS = 900;
const CLOSED_FADE_MS = 400;
const LOOK_AT = new THREE.Vector3(0, 0.25, -0.2);
const CAM_DIR = new THREE.Vector3(3.4, 2.2, 4.4).sub(LOOK_AT).normalize();
const BOLT_SVG_PATH = 'M18.2 4 10 17.5h5.4L13.8 28l8.2-13.5h-5.4L18.2 4z';

function easeInOutCubic(t) {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

function makeScreenTexture() {
  const canvas = document.createElement('canvas');
  canvas.width = 1024;
  canvas.height = 640;
  const ctx = canvas.getContext('2d');
  ctx.fillStyle = '#0d1117';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = '#161b22';
  ctx.fillRect(0, 0, canvas.width, 56);
  ctx.fillStyle = '#0a0a0a';
  ctx.fillRect(404, 0, 216, 34);
  ctx.font = '40px ui-monospace, "JetBrains Mono", Menlo, monospace';
  ctx.fillStyle = '#b8c5d6';
  ctx.fillText('$ npm test', 56, 140);
  ctx.fillStyle = '#3fb950';
  ctx.fillText('✓ 3 passing', 56, 220);
  ctx.fillStyle = '#c8941a';
  ctx.fillText('Done — 3 tests fixed', 56, 300);
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.needsUpdate = true;
  return tex;
}

function makeRadialSprite(innerAlpha, outerAlpha, color) {
  const canvas = document.createElement('canvas');
  const s = 256;
  canvas.width = s;
  canvas.height = s;
  const ctx = canvas.getContext('2d');
  const g = ctx.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2);
  const rgb = color || '0,0,0';
  g.addColorStop(0, 'rgba(' + rgb + ',' + innerAlpha + ')');
  g.addColorStop(1, 'rgba(' + rgb + ',' + outerAlpha + ')');
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, s, s);
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  return new THREE.Sprite(new THREE.SpriteMaterial({
    map: tex, transparent: true, depthWrite: false, toneMapped: false
  }));
}

function makeBoltSprite() {
  const canvas = document.createElement('canvas');
  canvas.width = 256;
  canvas.height = 256;
  const ctx = canvas.getContext('2d');
  ctx.fillStyle = '#c8941a';
  ctx.beginPath();
  ctx.arc(128, 128, 112, 0, Math.PI * 2);
  ctx.fill();
  ctx.save();
  ctx.translate(128, 128);
  ctx.scale(6.5, 6.5);
  ctx.translate(-16, -16);
  ctx.fillStyle = '#f4f6fa';
  ctx.fill(new Path2D(BOLT_SVG_PATH));
  ctx.restore();
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.needsUpdate = true;
  return new THREE.Sprite(new THREE.SpriteMaterial({
    map: tex, transparent: true, toneMapped: false
  }));
}

function fitCameraFramed(camera, laptop, lidGroup) {
  const savedRot = lidGroup.rotation.x;
  lidGroup.rotation.x = OPEN_ANGLE;
  laptop.updateMatrixWorld(true);
  const box = new THREE.Box3().setFromObject(laptop);
  const pts = [];
  const mx = box.min.x; const my = box.min.y; const mz = box.min.z;
  const Mx = box.max.x; const My = box.max.y; const Mz = box.max.z;
  for (const x of [mx, Mx]) {
    for (const y of [my, My]) {
      for (const z of [mz, Mz]) {
        pts.push(new THREE.Vector3(x, y, z));
      }
    }
  }
  lidGroup.rotation.x = savedRot;
  laptop.updateMatrixWorld(true);

  const lookAt = LOOK_AT.clone();
  let dist = 3.5;
  const targetW = 1.4;
  const targetBottom = 1 - 2 * 0.78;

  function measure(d, ly, lx) {
    lookAt.set(lx, ly, LOOK_AT.z);
    camera.position.copy(lookAt).addScaledVector(CAM_DIR, d);
    camera.lookAt(lookAt);
    let minX = Infinity; let maxX = -Infinity; let minY = Infinity;
    for (let i = 0; i < pts.length; i++) {
      const v = pts[i].clone().project(camera);
      minX = Math.min(minX, v.x);
      maxX = Math.max(maxX, v.x);
      minY = Math.min(minY, v.y);
    }
    return { w: maxX - minX, cx: (minX + maxX) * 0.5, minY };
  }

  for (let i = 0; i < 32; i++) {
    const m = measure(dist, lookAt.y, lookAt.x);
    if (m.w < targetW) dist *= 0.9;
    else dist *= 1.1;
  }

  let lookY = lookAt.y;
  for (let i = 0; i < 32; i++) {
    const m = measure(dist, lookY, lookAt.x);
    if (m.minY < targetBottom) lookY += 0.012;
    else lookY -= 0.012;
  }

  for (let i = 0; i < 24; i++) {
    const m = measure(dist, lookY, lookAt.x);
    lookAt.x -= m.cx * 0.12;
  }

  camera.position.copy(lookAt).addScaledVector(CAM_DIR, dist);
  camera.lookAt(lookAt);
  return lookAt;
}

function makeAlu(color) {
  return new THREE.MeshPhysicalMaterial({
    color: color,
    metalness: 0.9,
    roughness: 0.35,
    clearcoat: 0.25,
    clearcoatRoughness: 0.3,
    envMapIntensity: 1.1
  });
}

export function initLaptopDemo(canvas, onClosedChange, onFirstFrame) {
  if (!canvas) return Promise.reject(new Error('no canvas'));

  const renderer = new THREE.WebGLRenderer({
    canvas: canvas,
    alpha: true,
    antialias: true,
    powerPreference: 'high-performance'
  });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.05;
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = THREE.PCFSoftShadowMap;

  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(30, 1, 0.1, 100);

  const pmrem = new THREE.PMREMGenerator(renderer);
  let envReady = false;
  function applyEnvironment() {
    if (envReady) return;
    envReady = true;
    scene.environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture;
  }

  const key = new THREE.DirectionalLight(0xffffff, 1.35);
  key.position.set(3.5, 7, 4.5);
  key.castShadow = true;
  key.shadow.mapSize.set(2048, 2048);
  key.shadow.radius = 6;
  key.shadow.camera.near = 0.5;
  key.shadow.camera.far = 24;
  key.shadow.camera.left = -5;
  key.shadow.camera.right = 5;
  key.shadow.camera.top = 5;
  key.shadow.camera.bottom = -5;
  scene.add(key);

  const ground = new THREE.Mesh(
    new THREE.PlaneGeometry(24, 24),
    new THREE.ShadowMaterial({ opacity: 0.28 })
  );
  ground.rotation.x = -Math.PI / 2;
  ground.receiveShadow = true;
  scene.add(ground);

  const contactShadow = makeRadialSprite(0.35, 0);
  contactShadow.scale.set(3.4, 2.4, 1);
  contactShadow.position.set(0, 0.001, 0);
  scene.add(contactShadow);

  const baseMat = makeAlu(0x7a7c84);
  const lidMat = makeAlu(0x66686f);

  const laptop = new THREE.Group();
  scene.add(laptop);

  const base = new THREE.Mesh(new RoundedBoxGeometry(3.0, 0.10, 2.0, 4, 0.06), baseMat);
  base.position.y = 0.05;
  base.castShadow = true;
  base.receiveShadow = true;
  laptop.add(base);

  const DECK_TOP = 0.10;
  const WELL_DEPTH = 0.004;
  const WELL_FLOOR = DECK_TOP - WELL_DEPTH;
  const kbWell = new THREE.Mesh(
    new RoundedBoxGeometry(2.5, WELL_DEPTH, 0.8, 2, 0.002),
    new THREE.MeshStandardMaterial({ color: 0x141416, roughness: 0.95 })
  );
  kbWell.position.set(0, WELL_FLOOR + WELL_DEPTH * 0.5, 0.1);
  laptop.add(kbWell);

  const KEY_W = 0.13;
  const KEY_GAP = 0.012;
  const KEY_PITCH = KEY_W + KEY_GAP;
  const KEY_D = 0.11;
  const KEY_ROW_PITCH = KEY_D + KEY_GAP;
  const KEY_Y = WELL_FLOOR + 0.006;
  const keyGeo = new RoundedBoxGeometry(KEY_W, 0.006, KEY_D, 2, 0.008);
  const keyMat = new THREE.MeshStandardMaterial({ color: 0x1c1c1e, roughness: 0.9 });
  const keys = new THREE.InstancedMesh(keyGeo, keyMat, 70);
  let ki = 0;
  for (let row = 0; row < 5; row++) {
    for (let col = 0; col < 14; col++) {
      const m = new THREE.Matrix4();
      m.setPosition(-0.915 + col * KEY_PITCH, KEY_Y, -0.22 + row * KEY_ROW_PITCH);
      keys.setMatrixAt(ki++, m);
    }
  }
  keys.instanceMatrix.needsUpdate = true;
  keys.castShadow = true;
  laptop.add(keys);

  const trackpad = new THREE.Mesh(
    new THREE.PlaneGeometry(0.55, 0.36),
    new THREE.MeshStandardMaterial({ color: 0x3a3a3e, roughness: 0.85 })
  );
  trackpad.rotation.x = -Math.PI / 2;
  trackpad.position.set(0, DECK_TOP + 0.001, 0.55);
  laptop.add(trackpad);

  const footGeo = new THREE.CylinderGeometry(0.04, 0.04, 0.02, 12);
  [[-1.25, -0.75], [1.25, -0.75], [-1.25, 0.75], [1.25, 0.75]].forEach(function (p) {
    const foot = new THREE.Mesh(footGeo, baseMat);
    foot.position.set(p[0], 0.01, p[1]);
    laptop.add(foot);
  });

  const lidGroup = new THREE.Group();
  lidGroup.position.set(0, 0.10, -1.0);
  laptop.add(lidGroup);

  const lid = new THREE.Mesh(new RoundedBoxGeometry(3.0, 0.05, 1.95, 4, 0.06), lidMat);
  lid.position.set(0, 0.025, 0.975);
  lid.castShadow = true;
  lidGroup.add(lid);

  const screenTex = makeScreenTexture();
  const screenMat = new THREE.MeshBasicMaterial({
    map: screenTex,
    toneMapped: false,
    side: THREE.FrontSide,
    polygonOffset: true,
    polygonOffsetFactor: -4,
    polygonOffsetUnits: -4
  });
  const screen = new THREE.Mesh(new THREE.PlaneGeometry(2.84, 1.79), screenMat);
  screen.position.set(0, -0.024, 0);
  screen.rotation.x = Math.PI / 2;
  screen.renderOrder = 5;
  lid.add(screen);

  if (typeof console !== 'undefined' && console.log) {
    console.log('[demo-3d] screen texture', screenTex.image.width, 'x', screenTex.image.height);
    console.log('[demo-3d] screen material.map === texture', screenMat.map === screenTex);
    console.log('[demo-3d] screen side FrontSide', screenMat.side === THREE.FrontSide);
  }

  const notch = new THREE.Mesh(
    new THREE.PlaneGeometry(0.22, 0.04),
    new THREE.MeshBasicMaterial({ color: 0x0a0a0a, toneMapped: false, side: THREE.FrontSide })
  );
  notch.position.set(0, -0.023, -0.75);
  notch.rotation.x = Math.PI / 2;
  notch.renderOrder = 6;
  lid.add(notch);

  const hingeGlowLine = makeRadialSprite(0.4, 0, '200,148,26');
  hingeGlowLine.scale.set(3.2, 0.3, 1);
  hingeGlowLine.position.set(0, 0.101, -1.0);
  hingeGlowLine.material.opacity = 0;
  laptop.add(hingeGlowLine);

  const hingeGlow = new THREE.Mesh(
    new THREE.BoxGeometry(3.0, 0.02, 0.03),
    new THREE.MeshStandardMaterial({
      color: 0xc8941a,
      emissive: 0xc8941a,
      emissiveIntensity: 4,
      transparent: true,
      opacity: 0,
      toneMapped: false
    })
  );
  hingeGlow.position.set(0, 0.101, -1.0);
  laptop.add(hingeGlow);

  const bolt = makeBoltSprite();
  bolt.scale.set(0.45, 0.45, 1);
  bolt.position.set(0, 0.35, 0.05);
  bolt.material.opacity = 0;
  laptop.add(bolt);

  const boltShadow = makeRadialSprite(0.22, 0);
  boltShadow.scale.set(0.55, 0.22, 1);
  boltShadow.position.set(0, 0.12, 0.05);
  boltShadow.material.opacity = 0;
  laptop.add(boltShadow);

  lidGroup.rotation.x = OPEN_ANGLE;

  const controls = new OrbitControls(camera, canvas);
  controls.enableDamping = true;
  controls.dampingFactor = 0.06;
  controls.enableZoom = false;
  controls.enablePan = false;
  controls.autoRotate = false;
  controls.minPolarAngle = Math.PI * 0.18;
  controls.maxPolarAngle = Math.PI * 0.52;
  controls.minAzimuthAngle = -Math.PI * 0.4;
  controls.maxAzimuthAngle = Math.PI * 0.25;

  let closed = false;
  let anim = null;
  let closedFade = 0;
  let closedFadeTarget = 0;
  let effectT = 0;
  let clock = new THREE.Clock();
  let raf = 0;
  let disposed = false;
  let firstFrame = false;

  function resize() {
    const parent = canvas.parentElement;
    if (!parent) return;
    const w = parent.clientWidth;
    const h = w * 10 / 16;
    renderer.setSize(w, h, false);
    camera.aspect = w / h;
    camera.updateProjectionMatrix();
    controls.target.copy(fitCameraFramed(camera, laptop, lidGroup));
  }

  const ro = new ResizeObserver(resize);
  ro.observe(canvas.parentElement);
  resize();

  function setClosedVisual(isClosed, instant) {
    closed = isClosed;
    if (anim) cancelAnimationFrame(anim);
    const from = lidGroup.rotation.x;
    const to = isClosed ? CLOSED_ANGLE : OPEN_ANGLE;
    if (instant) {
      lidGroup.rotation.x = to;
      closedFadeTarget = isClosed ? 1 : 0;
      closedFade = closedFadeTarget;
      applyClosedState(isClosed);
      if (onClosedChange) onClosedChange(isClosed, true);
      return;
    }
    if (!isClosed) closedFadeTarget = 0;
    const start = performance.now();
    function tick(now) {
      const t = Math.min(1, (now - start) / LID_MS);
      const e = easeInOutCubic(t);
      lidGroup.rotation.x = from + (to - from) * e;
      screen.visible = !isClosed || e < 0.45;
      notch.visible = screen.visible;
      if (t < 1) {
        anim = requestAnimationFrame(tick);
      } else {
        anim = null;
        if (isClosed) closedFadeTarget = 1;
        if (onClosedChange) onClosedChange(isClosed, false);
      }
    }
    anim = requestAnimationFrame(tick);
  }

  function applyClosedState(isClosed) {
    const vis = closedFade > 0.01;
    hingeGlow.visible = vis;
    hingeGlowLine.visible = vis;
    bolt.visible = vis;
    boltShadow.visible = vis;
  }

  function render() {
    if (disposed) return;
    const dt = clock.getDelta();
    effectT += dt;
    controls.update();

    if (closedFade < closedFadeTarget) {
      closedFade = Math.min(closedFadeTarget, closedFade + dt / (CLOSED_FADE_MS / 1000));
    } else if (closedFade > closedFadeTarget) {
      closedFade = Math.max(closedFadeTarget, closedFade - dt / (CLOSED_FADE_MS / 1000));
    }

    if (closed) {
      const pulse = 0.7 + 0.3 * (0.5 + 0.5 * Math.sin(effectT * Math.PI));
      hingeGlow.material.opacity = closedFade * pulse;
      hingeGlowLine.material.opacity = closedFade * 0.4;
      bolt.material.opacity = closedFade;
      boltShadow.material.opacity = closedFade * 0.85;
      const bob = Math.sin(effectT * Math.PI) * 0.05;
      bolt.position.y = 0.35 + bob;
      boltShadow.position.y = 0.12 + bob * 0.3;
    } else {
      hingeGlow.material.opacity = 0;
      hingeGlowLine.material.opacity = 0;
      bolt.material.opacity = 0;
      boltShadow.material.opacity = 0;
    }
    applyClosedState(closed);

    renderer.render(scene, camera);
    if (!firstFrame) {
      firstFrame = true;
      if (onFirstFrame) onFirstFrame();
      var deferEnv = function () { applyEnvironment(); };
      if ('requestIdleCallback' in window) {
        requestIdleCallback(deferEnv, { timeout: 4000 });
      } else {
        setTimeout(deferEnv, 500);
      }
    }
    raf = requestAnimationFrame(render);
  }
  render();

  return Promise.resolve({
    setClosed: function (next, instant) { setClosedVisual(next, !!instant); },
    isClosed: function () { return closed; },
    capture: function () { render(); return canvas.toDataURL('image/png'); },
    destroy: function () {
      disposed = true;
      cancelAnimationFrame(raf);
      if (anim) cancelAnimationFrame(anim);
      ro.disconnect();
      controls.dispose();
      pmrem.dispose();
      renderer.dispose();
      screenTex.dispose();
      [bolt, boltShadow, contactShadow, hingeGlowLine].forEach(function (s) {
        if (s.material.map) s.material.map.dispose();
        s.material.dispose();
      });
    }
  });
}
