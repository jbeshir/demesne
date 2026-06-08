import { chromium } from 'playwright';
import { statSync, writeFileSync } from 'node:fs';

async function main() {
  const fixtureDir = process.env.FIXTURE_DIR ?? '/in/browser-fixture';
  const fileUrl = `file://${fixtureDir}/index.html`;

  const browser = await chromium.launch({
    headless: true,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-gpu',
    ],
  });

  const page = await browser.newPage();
  await page.goto(fileUrl, { waitUntil: 'load' });

  const text = await page.textContent('#root');
  if (!text || !text.includes('Hello from React')) {
    console.error(`text check failed: got ${JSON.stringify(text)}`);
    await browser.close();
    process.exit(1);
  }

  await page.screenshot({ path: '/out/screenshot.png', fullPage: true });

  const stat = statSync('/out/screenshot.png');
  if (stat.size === 0) {
    console.error('screenshot.png is empty');
    await browser.close();
    process.exit(1);
  }

  writeFileSync('/out/render-ok', 'ok\n');

  await browser.close();
  process.exit(0);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
