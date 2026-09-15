(function () {
  'use strict';
  var btn = document.getElementById('lid-btn');
  var stage = document.getElementById('demo-stage');
  var banner = document.getElementById('ios-banner');
  if (!btn || !stage) return;
  var closed = false;

  function setClosed(next) {
    closed = next;
    stage.classList.toggle('is-closed', closed);
    btn.textContent = closed ? 'Open the lid' : 'Close the lid';
    if (banner) banner.setAttribute('aria-hidden', closed ? 'false' : 'true');
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
