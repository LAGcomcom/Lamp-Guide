// The handwritten SVG is the single source for the UI, executable and installer.
const fs = require('node:fs');
const path = require('node:path');
const { Resvg } = require('../packaging/node_modules/@resvg/resvg-js');
const root = path.resolve(__dirname, '..');
const source = fs.readFileSync(path.join(root, 'web/assets/lamp-guide.svg'));
const output = path.join(root, 'packaging/assets');
fs.mkdirSync(output, { recursive: true });
const sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256];
const images = sizes.map(size => new Resvg(source, { fitTo: { mode: 'width', value: size } }).render().asPng());
const directory = Buffer.alloc(6 + sizes.length * 16);
directory.writeUInt16LE(1, 2);
directory.writeUInt16LE(sizes.length, 4);
let offset = directory.length;
sizes.forEach((size, i) => {
  const entry = 6 + i * 16;
  directory[entry] = directory[entry + 1] = size === 256 ? 0 : size;
  directory.writeUInt16LE(1, entry + 4);
  directory.writeUInt16LE(32, entry + 6);
  directory.writeUInt32LE(images[i].length, entry + 8);
  directory.writeUInt32LE(offset, entry + 12);
  offset += images[i].length;
});
fs.writeFileSync(path.join(output, 'lamp-guide.ico'), Buffer.concat([directory, ...images]));
fs.writeFileSync(path.join(output, 'lamp-guide.png'), images.at(-1));
function writeBitmap(name, svg) {
  const rendered = new Resvg(svg).render();
  const { width, height, pixels } = rendered;
  const stride = (width * 3 + 3) & ~3;
  const bitmap = Buffer.alloc(54 + stride * height);
  bitmap.write('BM');
  bitmap.writeUInt32LE(bitmap.length, 2);
  bitmap.writeUInt32LE(54, 10);
  bitmap.writeUInt32LE(40, 14);
  bitmap.writeInt32LE(width, 18);
  bitmap.writeInt32LE(height, 22);
  bitmap.writeUInt16LE(1, 26);
  bitmap.writeUInt16LE(24, 28);
  bitmap.writeUInt32LE(stride * height, 34);
  for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
    const src = (y * width + x) * 4;
    const dest = 54 + (height - 1 - y) * stride + x * 3;
    bitmap[dest] = pixels[src + 2];
    bitmap[dest + 1] = pixels[src + 1];
    bitmap[dest + 2] = pixels[src];
  }
  fs.writeFileSync(path.join(output, name), bitmap);
}
const iconData = 'data:image/png;base64,' + images.at(-1).toString('base64');
const mark = `<image x="44" y="108" width="240" height="240" href="${iconData}"/>`;
writeBitmap('wizard.bmp', `<svg xmlns="http://www.w3.org/2000/svg" width="328" height="628" viewBox="0 0 328 628">
  <rect width="328" height="628" fill="#101F30"/>
  <path d="M-40 520C70 390 260 600 370 390M-40 550C70 420 260 630 370 420" fill="none" stroke="#2B4358" stroke-width="2"/>
  ${mark}
  <text x="164" y="410" text-anchor="middle" font-family="Segoe UI" font-weight="600" font-size="36" fill="#FFF3D4">Lamp Guide</text>
  <path d="M130 446h68" stroke="#F4B54E" stroke-width="4" stroke-linecap="round"/>
  <circle cx="164" cy="560" r="4" fill="#F4B54E"/>
</svg>`);
writeBitmap('wizard-small.bmp', `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><rect width="64" height="64" fill="#FFFFFF"/><image width="64" height="64" href="${iconData}"/></svg>`);
console.log('Generated Lamp Guide icon: ' + sizes.join(', ') + ' px');
