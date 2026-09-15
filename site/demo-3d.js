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

function easeInOutCubic(t) {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

function makeScreenTexture() {
  const canvas = document.createElement('canvas');
  canvas.width = 640;
  canvas.height = 400;
  const ctx = canvas.getContext('2d');
  ctx.fillStyle = '#0d1117';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = '#161b22';
  ctx.fillRect(0, 0, canvas.width, 36);
  ctx.fillStyle = '#0a0a0a';
  ctx.fillRect(252, 0, 136, 22);
  ctx.font = '28px ui-monospace, "JetBrains Mono", Menlo, monospace';
  ctx.fillStyle = '#b8c5d6';
  ctx.fillText('$ npm test', 36, 88);
  ctx.fillStyle = '#3fb950';
  ctx.fillText('✓ 3 passing', 36, 138);
  ctx.fillStyle = '#c8941a';
  ctx.fillText('Done — 3 tests fixed', 36, 188);
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.needsUpdate = true;
  return tex;
}

function makeRadialSprite(size, innerAlpha, outerAlpha) {
  const canvas = document.createElement('canvas');
  const s = 256;
  canvas.width = s;
  canvas.height = s;
  const ctx = canvas.getContext('2d');
  const g = ctx.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2);
  g.addColorStop(0, 'rgba(0,0,0,' + innerAlpha + ')');
  g.addColorStop(1, 'rgba(0,0,0,' + outerAlpha + ')');
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
  canvas.width = 128;
  canvas.height = 128;
  const ctx = canvas.getContext('2d');
  ctx.fillStyle = '#c8941a';
  ctx.beginPath();
  ctx.arc(64, 64, 56, 0, Math.PI * 2);
  ctx.fill();
  ctx.fillStyle = '#f4f6fa';
  ctx.beginPath();
  ctx.moveTo(58, 92);
  ctx.lineTo(46, 58);
  ctx.lineTo(56, 58);
  ctx.lineTo(50, 36);
  ctx.lineTo(66, 36);
  ctx.lineTo(78, 70);
  ctx.lineTo(68, 70);
  ctx.lineTo(74, 92);
  ctx.closePath();
  ctx.fill();
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  return new THREE.Sprite(new THREE.SpriteMaterial({
    map: tex, transparent: true, toneMapped: false
  }));
}

function fitCamera(camera, object, aspect, margin) {
  const box = new THREE.Box3().setFromObject(object);
  const sphere = box.getBoundingSphere(new THREE.Sphere());
  const fovV = camera.fov * Math.PI / 180;
  const fovH = 2 * Math.atan(Math.tan(fovV / 2) * aspect);
  const distV = (sphere.radius * margin) / Math.sin(fovV / 2);
  const distH = (sphere.radius * margin) / Math.sin(fovH / 2);
  const dist = Math.max(distV, distH);
  camera.position.copy(LOOK_AT).addScaledVector(CAM_DIR, dist);
  camera.lookAt(LOOK_AT);
  return LOOK_AT.clone();
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

  const contactShadow = makeRadialSprite(1, 0.35, 0);
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

  const kbWell = new THREE.Mesh(
    new THREE.PlaneGeometry(2.5, 0.8),
    new THREE.MeshStandardMaterial({ color: 0x1c1c1e, roughness: 0.95 })
  );
  kbWell.rotation.x = -Math.PI / 2;
  kbWell.position.set(0, 0.0948, 0.1);
  laptop.add(kbWell);

  const keyGeo = new RoundedBoxGeometry(0.13, 0.006, 0.11, 2, 0.008);
  const keyMat = new THREE.MeshStandardMaterial({ color: 0x1c1c1e, roughness: 0.9 });
  const keys = new THREE.InstancedMesh(keyGeo, keyMat, 70);
  let ki = 0;
  for (let row = 0; row < 5; row++) {
    for (let col = 0; col < 14; col++) {
      const m = new THREE.Matrix4();
      m.setPosition(-0.91 + col * 0.135, 0.097, -0.22 + row * 0.13);
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
  trackpad.position.set(0, 0.0958, 0.55);
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
  const screen = new THREE.Mesh(
    new THREE.PlaneGeometry(2.84, 1.79),
    new THREE.MeshBasicMaterial({ map: screenTex, toneMapped: false })
  );
  screen.position.set(0, 0.024, 0.975);
  screen.rotation.x = -Math.PI / 2;
  screen.rotation.y = Math.PI;
  lidGroup.add(screen);

  const notch = new THREE.Mesh(
    new THREE.PlaneGeometry(0.22, 0.04),
    new THREE.MeshBasicMaterial({ color: 0x0a0a0a, toneMapped: false })
  );
  notch.position.set(0, 0.0255, 0.22);
  notch.rotation.x = -Math.PI / 2;
  notch.rotation.y = Math.PI;
  lidGroup.add(notch);

  const hingeGlow = new THREE.Mesh(
    new THREE.BoxGeometry(3.0, 0.02, 0.03),
    new THREE.MeshStandardMaterial({
      color: 0xc8941a,
      emissive: 0xc8941a,
      emissiveIntensity: 3,
      transparent: true,
      opacity: 0,
      toneMapped: false
    })
  );
  hingeGlow.position.set(0, 0.101, -1.0);
  laptop.add(hingeGlow);

  const bolt = makeBoltSprite();
  bolt.scale.set(0.5, 0.5, 1);
  bolt.position.set(0, 0.35, 0.05);
  bolt.material.opacity = 0;
  laptop.add(bolt);

  const boltShadow = makeRadialSprite(1, 0.22, 0);
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
    controls.target.copy(fitCamera(camera, laptop, w / h, 1.1));
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
      bolt.material.opacity = closedFade;
      boltShadow.material.opacity = closedFade * 0.85;
      const bob = Math.sin(effectT * Math.PI) * 0.05;
      bolt.position.y = 0.35 + bob;
      boltShadow.position.y = 0.12 + bob * 0.3;
    } else {
      hingeGlow.material.opacity = 0;
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
      [bolt, boltShadow, contactShadow].forEach(function (s) {
        if (s.material.map) s.material.map.dispose();
        s.material.dispose();
      });
    }
  });
}
