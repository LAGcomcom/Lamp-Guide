const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const modules = execFileSync('go', ['list', '-m', '-json', 'all'], { encoding: 'utf8' });
const used = new Set(execFileSync('go', ['list', '-deps', '-f', '{{with .Module}}{{.Path}}{{end}}', '.'], { encoding: 'utf8' }).split(/\r?\n/).filter(Boolean));
const records = [];
let start = -1, depth = 0, quoted = false, escaped = false;
for (let i = 0; i < modules.length; i++) {
  const char = modules[i];
  if (quoted) { if (escaped) escaped = false; else if (char === '\\') escaped = true; else if (char === '"') quoted = false; continue; }
  if (char === '"') { quoted = true; continue; }
  if (char === '{') { if (!depth) start = i; depth++; }
  if (char === '}' && --depth === 0) records.push(JSON.parse(modules.slice(start, i + 1)));
}
const output = [
  'Lamp Guide — Third-party notices',
  'This application continues the existing zen-ui / zen-gate project. Original source references:',
  'https://github.com/LAGcomcom/zen-gate',
  'Provider names and marks belong to their respective owners.',
  'Microsoft Edge WebView2 Runtime is distributed under Microsoft terms. Its signed installer is included as a shared runtime prerequisite; it is not part of Lamp Guide source.',
  'The Lamp Guide SVG lantern mark was created for this project.',
  'Lamp Guide / zen-gate source license:',
  fs.readFileSync(path.join(__dirname, '..', 'LICENSE'), 'utf8'),
  'Embedded QoderCN gateway license and upstream clarification:',
  fs.readFileSync(path.join(__dirname, '..', 'internal', 'zen', 'qoderhub', 'LICENSE'), 'utf8'),
  'Lane protocol reference license (dsh-our-free-model):',
  fs.readFileSync(path.join(__dirname, '..', 'licenses', 'dsh-our-free-model.txt'), 'utf8'),
  'Provider account / quota / aggregation reference license (dsh-providers-hub):',
  fs.readFileSync(path.join(__dirname, '..', 'licenses', 'dsh-providers-hub.txt'), 'utf8'),
  'The Go runtime and standard library are distributed under the following license:',
  fs.readFileSync(path.join(execFileSync('go', ['env', 'GOROOT'], { encoding: 'utf8' }).trim(), 'LICENSE'), 'utf8')
];
const bundled = records.filter(m => !m.Main && used.has(m.Path));
for (const module of bundled) {
  const dir = (module.Replace || module).Dir;
  if (!dir) throw Error('Missing module directory for ' + module.Path);
  const licenses = fs.readdirSync(dir).filter(name => /^(LICENSE|COPYING|NOTICE)(\..*)?$/i.test(name));
  if (!licenses.length) throw Error('Missing license for ' + module.Path);
  output.push('\n' + '='.repeat(72), module.Path + ' ' + module.Version);
  for (const file of licenses) output.push(file, fs.readFileSync(path.join(dir, file), 'utf8'));
}
fs.writeFileSync(process.argv[2], output.join('\r\n\r\n'), 'utf8');
console.log('Collected notices for ' + bundled.length + ' bundled Go modules.');
