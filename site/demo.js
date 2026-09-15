(function () {
  'use strict';
  var btn = document.getElementById('lid-btn');
  var stage = document.getElementById('demo-stage');
  var banner = document.getElementById('ios-banner');
  var fallback = document.getElementById('laptop-fallback');
  var canvas = document.getElementById('laptop-canvas');
  var laptopStage = document.getElementById('laptop-stage');
  if (!btn || !stage) return;

  var reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var closed = false;
  var laptop3d = null;
  var bannerTimer = null;
  var canLoad = false;
  var loading = false;
  var useFallback = reduced || !hasWebGL();

  function hasWebGL() {
    try {
      var c = document.createElement('canvas');
      return !!(window.WebGLRenderingContext && (c.getContext('webgl') || c.getContext('experimental-webgl')));
    } catch (e) { return false; }
  }

  function clearBannerTimer() {
    if (bannerTimer) { clearTimeout(bannerTimer); bannerTimer = null; }
  }

  function showBanner(show, instant) {
    clearBannerTimer();
    if (!banner) return;
    if (!show) {
      stage.classList.remove('show-banner');
      banner.setAttribute('aria-hidden', 'true');
      return;
    }
    if (instant) {
      stage.classList.add('show-banner');
      banner.setAttribute('aria-hidden', 'false');
      return;
    }
    bannerTimer = setTimeout(function () {
      stage.classList.add('show-banner');
      banner.setAttribute('aria-hidden', 'false');
    }, 600);
  }

  function applyClosed(next, instant) {
    closed = next;
    stage.classList.toggle('is-closed', closed);
    btn.textContent = closed ? 'Open the lid' : 'Close the lid';
    if (!closed) {
      showBanner(false, true);
      if (laptop3d) laptop3d.setClosed(false, instant);
      return;
    }
    if (laptop3d) {
      laptop3d.setClosed(true, instant);
    } else {
      showBanner(true, instant);
    }
  }

  function onLidClosed(isClosed, instant) {
    if (isClosed) showBanner(true, instant);
  }

  function revealCanvas() {
    if (!canvas || !fallback) return;
    fallback.hidden = true;
    canvas.hidden = false;
    stage.classList.add('is-3d');
  }

  function load3d() {
    if (laptop3d || useFallback || loading) return Promise.resolve(laptop3d);
    loading = true;
    return import('./demo-3d.js').then(function (mod) {
      return mod.initLaptopDemo(canvas, onLidClosed);
    }).then(function (api) {
      laptop3d = api;
      loading = false;
      revealCanvas();
      window.__laptopDemo = api;
      window.__laptopReady = true;
      return api;
    }).catch(function () {
      loading = false;
      useFallback = true;
      return null;
    });
  }

  function activate3d() {
    if (useFallback || !canLoad) return Promise.resolve(null);
    return load3d();
  }

  btn.addEventListener('click', function () {
    var next = !closed;
    if (!laptop3d && !useFallback && canLoad) {
      activate3d().then(function (api) {
        if (api) applyClosed(next, reduced);
        else applyClosed(next, reduced);
      });
      return;
    }
    applyClosed(next, reduced);
  });

  if (!useFallback && laptopStage && canvas) {
    if ('IntersectionObserver' in window) {
      var obs = new IntersectionObserver(function (entries) {
        if (entries[0].isIntersecting) {
          canLoad = true;
          obs.disconnect();
        }
      }, { rootMargin: '200px' });
      obs.observe(laptopStage);
    } else {
      canLoad = true;
    }
    laptopStage.addEventListener('pointerdown', function () {
      if (!laptop3d && canLoad) activate3d();
    }, { once: true });
  }

  document.querySelectorAll('.cmd-wrap pre code').forEach(function (code) {
    var wrap = code.closest('.cmd-wrap');
    if (!wrap) return;
    var b = document.createElement('button');
    b.className = 'copy-btn';
    b.type = 'button';
    b.textContent = 'copy';
    b.setAttribute('aria-label', 'Copy command');
    b.addEventListener('click', function () {
      navigator.clipboard.writeText(code.textContent).then(function () {
        b.textContent = 'copied';
        window.setTimeout(function () { b.textContent = 'copy'; }, 1200);
      });
    });
    wrap.appendChild(b);
  });
})();
