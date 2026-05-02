#!/usr/bin/env node
import { chromium } from 'playwright';
import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

function arg(name, fallback) {
  const idx = process.argv.indexOf(`--${name}`);
  return idx >= 0 && process.argv[idx + 1] ? process.argv[idx + 1] : fallback;
}

const root = process.cwd();
const artifactsDir = path.resolve(root, arg('artifacts', 'emulator-artifacts'));
const featureDir = path.resolve(root, arg('features', 'features/ux'));
const outDir = path.resolve(root, arg('out', path.join(artifactsDir, 'ux-report')));

const requiredFiles = [
  'checks.txt',
  'input-validation-plan.txt',
  'rdp-probe-summary.json',
  'rdp-home.png',
  'rdp-input-settings-search.png',
  'rdp-input-mouse-target.png',
  'rdp-input-notifications.png',
  'rdp-browser.png',
];

async function exists(file) {
  try {
    await fs.access(file);
    return true;
  } catch {
    return false;
  }
}

async function readText(file, fallback = '') {
  try {
    return await fs.readFile(file, 'utf8');
  } catch {
    return fallback;
  }
}

async function readJSON(file) {
  return JSON.parse(await fs.readFile(file, 'utf8'));
}

function escapeHtml(s) {
  return String(s)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

function markdownEscape(s) {
  return String(s).replaceAll('|', '\\|');
}

async function parseFeatures(dir) {
  const files = (await fs.readdir(dir)).filter((f) => f.endsWith('.feature')).sort();
  const scenarios = [];
  for (const file of files) {
    const full = path.join(dir, file);
    const lines = (await fs.readFile(full, 'utf8')).split(/\r?\n/);
    let feature = '';
    let current = null;
    for (const line of lines) {
      const trimmed = line.trim();
      if (trimmed.startsWith('Feature:')) feature = trimmed.replace(/^Feature:\s*/, '');
      if (trimmed.startsWith('Scenario:')) {
        if (current) scenarios.push(current);
        current = { file, feature, name: trimmed.replace(/^Scenario:\s*/, ''), steps: [] };
      } else if (current && /^(Given|When|Then|And|But)\b/.test(trimmed)) {
        current.steps.push(trimmed);
      }
    }
    if (current) scenarios.push(current);
  }
  return scenarios;
}

function scenarioId(name) {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '');
}

function evidenceForScenario(scenario, checks, inputPlan, summary) {
  const name = scenario.name.toLowerCase();
  const scenes = summary.scenes || [];
  const sceneByName = Object.fromEntries(scenes.map((s) => [s.name, s]));
  const evidence = [];
  const screenshots = [];
  const metrics = [];
  const addCheck = (label, ok, detail = '') => evidence.push({ label, ok, detail });
  const addShot = (label, filename) => screenshots.push({ label, filename });

  if (name.includes('start')) {
    addCheck('RDP backend started', checks.includes('startServer=ok'));
    addCheck('First frame submitted', checks.includes('frame1=ok'));
    addCheck('MediaProjection capture active', checks.includes('screen_capture=ok'));
    addCheck('No fatal Android exception', checks.includes('fatal_exception=none'));
    addShot('RDP home', 'rdp-home.png');
    metrics.push(['bitmap_updates', summary.bitmap_updates]);
  } else if (name.includes('search android settings')) {
    addCheck('Keyboard search step passed', checks.includes('keyboard_settings_search=ok'));
    addCheck('Settings search scene recorded', Boolean(sceneByName['settings-search']));
    addShot('RDP Settings search', 'rdp-input-settings-search.png');
    addShot('Android Settings search', 'android-input-settings-search.png');
    if (sceneByName['settings-search']) metrics.push(['settings-search updates', sceneByName['settings-search'].updates]);
  } else if (name.includes('mouse-source')) {
    addCheck('Mouse tap step passed', checks.includes('mouse_target_tap=ok'));
    addCheck('Mouse coordinates recorded', /^mouse=/m.test(inputPlan), inputPlan.match(/^mouse=.*$/m)?.[0] || '');
    addShot('RDP mouse target', 'rdp-input-mouse-target.png');
    addShot('Android mouse target', 'android-input-mouse-target.png');
    if (sceneByName['mouse-target']) metrics.push(['mouse-target updates', sceneByName['mouse-target'].updates]);
  } else if (name.includes('notifications')) {
    addCheck('Touch swipe step passed', checks.includes('touch_notification_swipe=ok'));
    addCheck('Swipe coordinates recorded', /^touch=/m.test(inputPlan), inputPlan.match(/^touch=.*$/m)?.[0] || '');
    addShot('RDP notifications', 'rdp-input-notifications.png');
    addShot('Android notifications', 'android-notifications.png');
    if (sceneByName.notifications) metrics.push(['notifications updates', sceneByName.notifications.updates]);
  } else if (name.includes('browser')) {
    addCheck('Browser intent launched', /Activity: .*chrome|Activity: .*browser/i.test(summary._browserStart || ''));
    addShot('RDP browser', 'rdp-browser.png');
    addShot('Android browser', 'android-browser.png');
    if (sceneByName.browser) metrics.push(['browser updates', sceneByName.browser.updates]);
  } else if (name.includes('performance')) {
    addCheck('Performance summary JSON present', Number.isFinite(summary.bitmap_updates));
    addCheck('Per-scene metrics present', scenes.length >= 3, `${scenes.length} scenes`);
    metrics.push(['total bitmap updates', summary.bitmap_updates]);
    metrics.push(['bitmap payload bytes', summary.bitmap_payload_bytes]);
    metrics.push(['duration ms', summary.duration_ms]);
    addShot('RDP browser final', 'rdp-browser.png');
  }

  return { evidence, screenshots, metrics };
}

async function imageInfo(page, file) {
  const full = path.join(artifactsDir, file);
  if (!(await exists(full))) return { exists: false, width: 0, height: 0 };
  await page.goto(`file://${full}`);
  const box = await page.evaluate(() => ({ width: document.images[0]?.naturalWidth || 0, height: document.images[0]?.naturalHeight || 0 }));
  return { exists: true, ...box };
}

async function imageDataUri(file) {
  const full = path.join(artifactsDir, file);
  const data = await fs.readFile(full);
  return `data:image/png;base64,${data.toString('base64')}`;
}

async function main() {
  await fs.mkdir(outDir, { recursive: true });
  const missing = [];
  for (const f of requiredFiles) {
    if (!(await exists(path.join(artifactsDir, f)))) missing.push(f);
  }
  if (missing.length) throw new Error(`Missing UX artifacts: ${missing.join(', ')}`);

  const scenarios = await parseFeatures(featureDir);
  if (!scenarios.length) throw new Error(`No feature scenarios found in ${featureDir}`);
  const checks = await readText(path.join(artifactsDir, 'checks.txt'));
  const inputPlan = await readText(path.join(artifactsDir, 'input-validation-plan.txt'));
  const summary = await readJSON(path.join(artifactsDir, 'rdp-probe-summary.json'));
  summary._browserStart = await readText(path.join(artifactsDir, 'browser-start.txt'));

  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  const results = [];
  for (const scenario of scenarios) {
    const validation = evidenceForScenario(scenario, checks, inputPlan, summary);
    for (const shot of validation.screenshots) {
      shot.info = await imageInfo(page, shot.filename);
      validation.evidence.push({
        label: `${shot.label} screenshot exists`,
        ok: shot.info.exists && shot.info.width > 0 && shot.info.height > 0,
        detail: shot.info.exists ? `${shot.info.width}x${shot.info.height}` : shot.filename,
      });
    }
    const ok = validation.evidence.length > 0 && validation.evidence.every((e) => e.ok);
    results.push({ scenario, ...validation, ok });
  }

  const failed = results.filter((r) => !r.ok);
  const md = [];
  md.push('# Android RDP UX Test Report');
  md.push('');
  md.push(`Generated: ${new Date().toISOString()}`);
  md.push('');
  md.push(`Feature scenarios: ${results.length}`);
  md.push(`Status: ${failed.length === 0 ? 'PASS' : 'FAIL'}`);
  md.push('');
  md.push('| Scenario | Status |');
  md.push('| --- | --- |');
  for (const r of results) md.push(`| ${markdownEscape(r.scenario.name)} | ${r.ok ? 'PASS' : 'FAIL'} |`);
  md.push('');

  let html = `<!doctype html><html><head><meta charset="utf-8"><title>Android RDP UX Test Report</title><style>
    body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;margin:32px;color:#111;}
    h1{font-size:28px} h2{break-before:page;font-size:22px;margin-top:0} h3{font-size:16px;margin-bottom:8px}
    .summary{padding:12px 16px;background:#eef6ff;border-left:4px solid #2271b1;margin:16px 0}
    .pass{color:#107c10;font-weight:700}.fail{color:#b00020;font-weight:700}
    table{border-collapse:collapse;width:100%;margin:12px 0}td,th{border:1px solid #ddd;padding:6px;text-align:left;font-size:12px}
    pre{white-space:pre-wrap;background:#f6f8fa;padding:10px;border-radius:6px;font-size:11px}
    img{max-width:100%;border:1px solid #ddd;margin:8px 0}.shots{display:grid;grid-template-columns:1fr 1fr;gap:12px}.shot{break-inside:avoid}
  </style></head><body>`;
  html += `<h1>Android RDP UX Test Report</h1><div class="summary"><p><b>Generated:</b> ${new Date().toISOString()}</p><p><b>Status:</b> <span class="${failed.length ? 'fail' : 'pass'}">${failed.length ? 'FAIL' : 'PASS'}</span></p><p><b>Feature scenarios:</b> ${results.length}</p></div>`;

  for (const r of results) {
    md.push(`## ${r.ok ? 'PASS' : 'FAIL'} — ${r.scenario.name}`);
    md.push('');
    md.push(`Feature: ${r.scenario.feature}`);
    md.push('');
    md.push('```gherkin');
    md.push(`Scenario: ${r.scenario.name}`);
    for (const step of r.scenario.steps) md.push(`  ${step}`);
    md.push('```');
    md.push('');
    md.push('| Evidence | Status | Detail |');
    md.push('| --- | --- | --- |');
    for (const e of r.evidence) md.push(`| ${markdownEscape(e.label)} | ${e.ok ? 'PASS' : 'FAIL'} | ${markdownEscape(e.detail || '')} |`);
    md.push('');
    if (r.metrics.length) {
      md.push('| Metric | Value |');
      md.push('| --- | ---: |');
      for (const [k, v] of r.metrics) md.push(`| ${markdownEscape(k)} | ${v ?? ''} |`);
      md.push('');
    }
    for (const shot of r.screenshots) md.push(`![${shot.label}](../${shot.filename})`);
    md.push('');

    html += `<section><h2><span class="${r.ok ? 'pass' : 'fail'}">${r.ok ? 'PASS' : 'FAIL'}</span> — ${escapeHtml(r.scenario.name)}</h2>`;
    html += `<p><b>Feature:</b> ${escapeHtml(r.scenario.feature)}</p><h3>Gherkin</h3><pre>Scenario: ${escapeHtml(r.scenario.name)}\n${r.scenario.steps.map((s) => `  ${escapeHtml(s)}`).join('\n')}</pre>`;
    html += '<h3>Evidence</h3><table><thead><tr><th>Evidence</th><th>Status</th><th>Detail</th></tr></thead><tbody>';
    for (const e of r.evidence) html += `<tr><td>${escapeHtml(e.label)}</td><td class="${e.ok ? 'pass' : 'fail'}">${e.ok ? 'PASS' : 'FAIL'}</td><td>${escapeHtml(e.detail || '')}</td></tr>`;
    html += '</tbody></table>';
    if (r.metrics.length) {
      html += '<h3>Metrics</h3><table><tbody>';
      for (const [k, v] of r.metrics) html += `<tr><td>${escapeHtml(k)}</td><td>${escapeHtml(v ?? '')}</td></tr>`;
      html += '</tbody></table>';
    }
    html += '<div class="shots">';
    for (const shot of r.screenshots) {
      if (shot.info?.exists) html += `<div class="shot"><h3>${escapeHtml(shot.label)}</h3><img src="${await imageDataUri(shot.filename)}"><p>${escapeHtml(shot.filename)} — ${shot.info.width}x${shot.info.height}</p></div>`;
    }
    html += '</div></section>';
  }
  html += '</body></html>';

  const mdPath = path.join(outDir, 'ux-report.md');
  const htmlPath = path.join(outDir, 'ux-report.html');
  const pdfPath = path.join(outDir, 'ux-report.pdf');
  await fs.writeFile(mdPath, md.join('\n'));
  await fs.writeFile(htmlPath, html);
  await fs.writeFile(path.join(outDir, 'ux-validation.json'), JSON.stringify({ ok: failed.length === 0, results }, null, 2));

  await page.setContent(html, { waitUntil: 'load' });
  await page.pdf({ path: pdfPath, format: 'A4', printBackground: true, margin: { top: '12mm', right: '10mm', bottom: '12mm', left: '10mm' } });
  await browser.close();

  console.log(`UX report written to ${pdfPath}`);
  if (failed.length) {
    for (const f of failed) console.error(`FAILED: ${f.scenario.name}`);
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
