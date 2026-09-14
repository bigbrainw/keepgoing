(function () {
  'use strict';
  var btn = document.getElementById('lid-btn');
  var stage = document.getElementById('demo-stage');
  if (!btn || !stage) return;
  var reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var closed = false;

  btn.addEventListener('click', function () {
    closed = !closed;
    stage.classList.toggle('is-closed', closed);
    btn.textContent = closed ? 'Open the lid' : 'Close the lid';
    if (!reduced) {
      stage.classList.add('is-animating');
      window.setTimeout(function () { stage.classList.remove('is-animating'); }, 600);
    }
  });

  document.querySelectorAll('.cmd-wrap pre code').forEach(function (code) {
    var wrap = code.closest('.cmd-wrap');
    if (!wrap) return;
    var b = document.createElement('button');
    b.className = 'copy-btn';
    b.type = 'button';
    b.textContent = 'copy';
    b.addEventListener('click', function () {
      navigator.clipboard.writeText(code.textContent).then(function () {
        b.textContent = 'copied';
        window.setTimeout(function () { b.textContent = 'copy'; }, 1200);
      });
    });
    wrap.appendChild(b);
  });
})();
