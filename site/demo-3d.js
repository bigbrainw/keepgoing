import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js';
import { RoundedBoxGeometry } from 'three/addons/geometries/RoundedBoxGeometry.js';

const OPEN_ANGLE = -100 * (Math.PI / 180);
const CLOSED_ANGLE = 0;
const LID_MS = 900;

function easeInOutCubic(t) {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

function makeScreenTexture() {
  const canvas = document.createElement('canvas');
  canvas.width = 512;
  canvas.height = 320;
  const ctx = canvas.getContext('2d');
  ctx.fillStyle = '#0d1117';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = '#1a2332';
  ctx.fillRect(0, 0, canvas.width, 28);
  ctx.fillStyle = '#1c1c1e';
  ctx.fillRect(200, 0, 112, 18);
  ctx.font = '22px "JetBrains Mono", ui-monospace, Menlo, monospace';
  ctx.fillStyle = '#8b9cb3';
  const lines = ['$ npm test', '3 passing', 'Done — 3 tests fixed'];
  lines.forEach(function (line, i) {
    ctx.fillStyle = i === 2 ? '#c8941a' : '#b8c5d6';
    ctx.fillText(line, 28, 72 + i * 34);
  });
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  return tex;
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
  return new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true }));
}

export function initLaptopDemo(canvas, onClosedChange) {
  if (!canvas) return Promise.reject(new Error('no canvas'));

  var renderer = new THREE.WebGLRenderer({
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

  var scene = new THREE.Scene();
  var camera = new THREE.PerspectiveCamera(32, 1, 0.1, 100);
  camera.position.set(-2.8, 1.65, 2.6);
  camera.lookAt(0, 0.35, 0);

  var pmrem = new THREE.PMREMGenerator(renderer);
  scene.environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture;

  var key = new THREE.DirectionalLight(0xffffff, 1.2);
  key.position.set(-4, 6, 3);
  key.castShadow = true;
  key.shadow.mapSize.set(1024, 1024);
  key.shadow.camera.near = 0.5;
  key.shadow.camera.far = 20;
  key.shadow.camera.left = -4;
  key.shadow.camera.right = 4;
  key.shadow.camera.top = 4;
  key.shadow.camera.bottom = -4;
  scene.add(key);

  var ground = new THREE.Mesh(
    new THREE.PlaneGeometry(20, 20),
    new THREE.ShadowMaterial({ opacity: 0.18 })
  );
  ground.rotation.x = -Math.PI / 2;
  ground.receiveShadow = true;
  scene.add(ground);

  var alu = new THREE.MeshPhysicalMaterial({
    color: 0x6e7077,
    metalness: 0.85,
    roughness: 0.45,
    clearcoat: 0.15
  });

  var laptop = new THREE.Group();
  scene.add(laptop);

  var base = new THREE.Mesh(new RoundedBoxGeometry(3.0, 0.10, 2.0, 4, 0.06), alu);
  base.position.y = 0.05;
  base.castShadow = true;
  base.receiveShadow = true;
  laptop.add(base);

  var kbWell = new THREE.Mesh(
    new THREE.PlaneGeometry(2.55, 1.35),
    new THREE.MeshStandardMaterial({ color: 0x1c1c1e, roughness: 0.9 })
  );
  kbWell.rotation.x = -Math.PI / 2;
  kbWell.position.set(0, 0.0945, 0.08);
  laptop.add(kbWell);

  var trackpad = new THREE.Mesh(
    new THREE.PlaneGeometry(0.55, 0.36),
    new THREE.MeshStandardMaterial({ color: 0x3a3a3e, roughness: 0.85 })
  );
  trackpad.rotation.x = -Math.PI / 2;
  trackpad.position.set(0, 0.0955, 0.52);
  laptop.add(trackpad);

  var footGeo = new THREE.CylinderGeometry(0.04, 0.04, 0.02, 12);
  [[-1.25, -0.75], [1.25, -0.75], [-1.25, 0.75], [1.25, 0.75]].forEach(function (p) {
    var foot = new THREE.Mesh(footGeo, alu);
    foot.position.set(p[0], 0.01, p[1]);
    laptop.add(foot);
  });

  var lidGroup = new THREE.Group();
  lidGroup.position.set(0, 0.10, -1.0);
  laptop.add(lidGroup);

  var lid = new THREE.Mesh(new RoundedBoxGeometry(3.0, 0.05, 1.95, 4, 0.06), alu);
  lid.position.set(0, 0.025, 0.975);
  lid.castShadow = true;
  lidGroup.add(lid);

  var screenTex = makeScreenTexture();
  var screen = new THREE.Mesh(
    new THREE.PlaneGeometry(2.84, 1.79),
    new THREE.MeshBasicMaterial({ map: screenTex })
  );
  screen.position.set(0, 0.026, 0.975);
  screen.rotation.x = -Math.PI / 2;
  lidGroup.add(screen);

  var bloom = new THREE.Mesh(
    new THREE.PlaneGeometry(2.9, 1.85),
    new THREE.MeshBasicMaterial({ color: 0x223044, transparent: true, opacity: 0.35 })
  );
  bloom.position.copy(screen.position);
  bloom.position.y -= 0.002;
  bloom.rotation.copy(screen.rotation);
  lidGroup.add(bloom);

  var notch = new THREE.Mesh(
    new THREE.PlaneGeometry(0.22, 0.04),
    new THREE.MeshBasicMaterial({ color: 0x0a0a0a })
  );
  notch.position.set(0, 0.027, 0.22);
  notch.rotation.x = -Math.PI / 2;
  lidGroup.add(notch);

  var hingeGlow = new THREE.Mesh(
    new THREE.BoxGeometry(3.0, 0.004, 0.02),
    new THREE.MeshStandardMaterial({
      color: 0xc8941a,
      emissive: 0xc8941a,
      emissiveIntensity: 2,
      transparent: true,
      opacity: 0
    })
  );
  hingeGlow.position.set(0, 0.102, -1.0);
  laptop.add(hingeGlow);

  var bolt = makeBoltSprite();
  bolt.scale.set(0.28, 0.28, 1);
  bolt.position.set(0, 0.35, 0.2);
  bolt.material.opacity = 0;
  laptop.add(bolt);

  lidGroup.rotation.x = OPEN_ANGLE;

  var controls = new OrbitControls(camera, canvas);
  controls.enableDamping = true;
  controls.dampingFactor = 0.06;
  controls.enableZoom = false;
  controls.enablePan = false;
  controls.autoRotate = false;
  controls.minPolarAngle = Math.PI * 0.22;
  controls.maxPolarAngle = Math.PI * 0.48;
  controls.minAzimuthAngle = -Math.PI * 0.35;
  controls.maxAzimuthAngle = Math.PI * 0.2;
  controls.target.set(0, 0.35, 0);

  var closed = false;
  var anim = null;
  var bobT = 0;
  var clock = new THREE.Clock();
  var raf = 0;
  var disposed = false;

  function resize() {
    var parent = canvas.parentElement;
    if (!parent) return;
    var w = parent.clientWidth;
    var h = w * 10 / 16;
    renderer.setSize(w, h, false);
    camera.aspect = w / h;
    camera.updateProjectionMatrix();
  }

  var ro = new ResizeObserver(resize);
  ro.observe(canvas.parentElement);
  resize();

  function setClosedVisual(isClosed, instant) {
    closed = isClosed;
    if (anim) cancelAnimationFrame(anim);
    var from = lidGroup.rotation.x;
    var to = isClosed ? CLOSED_ANGLE : OPEN_ANGLE;
    if (instant) {
      lidGroup.rotation.x = to;
      applyClosedState(isClosed, 1);
      if (onClosedChange) onClosedChange(isClosed, true);
      return;
    }
    var start = performance.now();
    function tick(now) {
      var t = Math.min(1, (now - start) / LID_MS);
      var e = easeInOutCubic(t);
      lidGroup.rotation.x = from + (to - from) * e;
      applyClosedState(isClosed, isClosed ? e : 1 - e);
      if (t < 1) {
        anim = requestAnimationFrame(tick);
      } else {
        anim = null;
        if (onClosedChange) onClosedChange(isClosed, false);
      }
    }
    anim = requestAnimationFrame(tick);
  }

  function applyClosedState(isClosed, blend) {
    var b = Math.max(0, Math.min(1, blend));
    screen.visible = !isClosed || b < 0.5;
    bloom.visible = screen.visible;
    notch.visible = screen.visible;
    hingeGlow.material.opacity = isClosed ? b : 0;
    bolt.material.opacity = isClosed ? b : 0;
  }

  function render() {
    if (disposed) return;
    var dt = clock.getDelta();
    controls.update();
    if (closed) {
      bobT += dt;
      bolt.position.y = 0.35 + Math.sin(bobT * 2.2) * 0.03;
    }
    renderer.render(scene, camera);
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
      if (bolt.material.map) bolt.material.map.dispose();
      bolt.material.dispose();
    }
  });
}
