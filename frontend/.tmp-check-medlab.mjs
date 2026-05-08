import { chromium } from 'playwright';
const browser = await chromium.launch({ headless: false });
const page = await browser.newPage();
const errors = [];
page.on('console', msg => { if (['error','warning'].includes(msg.type())) errors.push(`${msg.type()}: ${msg.text()}`); });
page.on('response', async res => {
  const url = res.url();
  if (url.includes('localhost:8100') || url.includes('127.0.0.1:8100')) {
    if (res.status() >= 400) errors.push(`${res.status()} ${url} ${await res.text().catch(()=> '')}`);
  }
});
await page.goto('http://127.0.0.1:5180/medlab/dashboard', { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(8000);
console.log('URL=', page.url());
console.log('TEXT=', (await page.locator('body').innerText().catch(e => String(e))).slice(0, 2000));
console.log('ERRORS=', errors.join('\n'));
await page.screenshot({ path: '/tmp/pymes-medlab-dashboard-check.png', fullPage: true });
await browser.close();
