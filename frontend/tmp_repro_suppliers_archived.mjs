import { chromium } from '@playwright/test';
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.on('request', req => {
  const u = req.url();
  if (u.includes('/v1/suppliers')) console.log('REQ', req.method(), u);
});
page.on('response', async res => {
  const u = res.url();
  if (u.includes('/v1/suppliers')) {
    let body='';
    try { body = await res.text(); } catch {}
    console.log('RES', res.status(), u, body.slice(0, 400));
  }
});
page.on('console', msg => console.log('CONSOLE', msg.type(), msg.text()));
await page.goto('http://127.0.0.1:5180/bicimax/suppliers?archived=1', { waitUntil: 'networkidle', timeout: 60000 });
console.log('URL', page.url());
console.log('HAS HEADING', await page.getByRole('heading', { name: /Proveedores archivados/i }).isVisible().catch(()=>false));
console.log('BODY', (await page.locator('body').innerText()).slice(0,2000));
await page.screenshot({ path:'/tmp/suppliers-archived.png', fullPage:true });
await browser.close();
