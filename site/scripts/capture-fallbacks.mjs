#!/usr/bin/env node
import { spawn } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const outDir = join(root, 'img');

function dataUrlToBuffer(dataUrl) {
  return Buffer.from(dataUrl.replace(/^data:image\/png;base64,/, ''), 'base64');
}

const server = spawn('python3', ['-m', 'http.server', '8790'], { cwd: root, stdio: 'ignore' });
await new Promise((r) => setTimeout(r, 800));

try {
  const { chromium } = await import('playwright');
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1600, height: 1100 } });
  await page.goto('http://127.0.0.1:8790/');
  await page.locator('#laptop-stage').scrollIntoViewIfNeeded();
  await page.waitForFunction(() => window.__laptopReady, null, { timeout: 30000 });

  const openUrl = await page.evaluate(async () => {
    window.__laptopDemo.setClosed(false, true);
    await new Promise((r) => setTimeout(r, 300));
    return window.__laptopDemo.capture();
  });
  writeFileSync(join(outDir, 'laptop-open.png'), dataUrlToBuffer(openUrl));

  const closedUrl = await page.evaluate(async () => {
    window.__laptopDemo.setClosed(true, true);
    await new Promise((r) => setTimeout(r, 300));
    return window.__laptopDemo.capture();
  });
  writeFileSync(join(outDir, 'laptop-closed.png'), dataUrlToBuffer(closedUrl));

  await browser.close();
  console.log('wrote img/laptop-open.png and img/laptop-closed.png');
} finally {
  server.kill();
}
