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

  btn.addEventListener('click', function () {
    applyClosed(!closed, reduced);
  });

  function revealCanvas() {
    if (!canvas || !fallback) return;
    fallback.hidden = true;
    canvas.hidden = false;
    stage.classList.add('is-3d');
  }

  function load3d() {
    if (laptop3d || useFallback) return;
    import('./demo-3d.js').then(function (mod) {
      return mod.initLaptopDemo(canvas, onLidClosed);
    }).then(function (api) {
      laptop3d = api;
      revealCanvas();
      window.__laptopDemo = api;
      window.__laptopReady = true;
    }).catch(function () {
      useFallback = true;
    });
  }

  if (!useFallback && laptopStage && canvas) {
    if ('IntersectionObserver' in window) {
      var obs = new IntersectionObserver(function (entries) {
        if (entries[0].isIntersecting) {
          obs.disconnect();
          load3d();
        }
      }, { rootMargin: '200px' });
      obs.observe(laptopStage);
    } else {
      load3d();
    }
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
