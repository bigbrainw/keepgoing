(function () {
  'use strict';
  var btn = document.getElementById('lid-btn');
  var stage = document.getElementById('demo-stage');
  if (!btn || !stage) return;
  var reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var closed = false;
  var timers = [];

  function clearTimers() {
    timers.forEach(clearTimeout);
    timers = [];
  }

  function setClosed(next) {
    closed = next;
    clearTimers();
    stage.classList.toggle('is-closed', closed);
    stage.classList.remove('phase-typing', 'phase-done');
    btn.textContent = closed ? 'Open the lid' : 'Close the lid';
    if (!closed) return;
    if (reduced) {
      stage.classList.add('phase-typing', 'phase-done');
      return;
    }
    timers.push(setTimeout(function () {
      stage.classList.add('phase-typing');
    }, 900));
    timers.push(setTimeout(function () {
      stage.classList.add('phase-done');
    }, 2400));
  }

  btn.addEventListener('click', function () {
    setClosed(!closed);
  });

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
