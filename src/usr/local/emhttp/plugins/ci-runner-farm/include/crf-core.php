<?php
/* Shared CI Runner Farm web core — the crf* JS helpers, the @unraid/ui force-loader,
   and the shared .crf-* styles used by ALL three RunnerFarm tabs (Fleet/Image/
   Settings). include_once'd from the top of each tab so the dependency is EXPLICIT
   and load-order-independent, instead of living inside the Fleet tab and being
   relied on by document order (renaming crfPost or reordering the tab ordinals used
   to silently break the other tabs). Emitted once per document via include_once.
   Runs in the tab's scope, so $var (the CSRF token) is available. */
/* Dynamix loads every RunnerFarm tab into ONE document, so a `const` emitted by two
   tabs is a redeclaration SyntaxError that kills both scripts. CRF_DEFAULTS is needed
   by the Fleet tab (fleet keys) and the Settings tab (global keys) alike, so it is
   emitted here — the one place that is include_once'd by all of them. */
include_once __DIR__ . '/crf-config.php';
$crf_csrf = $var['csrf_token'] ?? '';
$crf_uui_base = '/plugins/dynamix.my.servers/unraid-components/uui/';
$crf_util_css = '';
foreach (glob('/usr/local/emhttp/plugins/dynamix.my.servers/unraid-components/standalone/standalone-apps-*.css') ?: [] as $f) {
  $crf_util_css = '/plugins/dynamix.my.servers/unraid-components/standalone/' . basename($f);
  break;
}
?>
<style>
  :root{--crf-ok:#4caf50;--crf-busy:var(--brand-orange,#ff8c2f);--crf-err:var(--brand-red,#e22828);--crf-info:var(--link-text-color,#29b6f6)}
  .crf-muted{color:var(--alt-text-color)}
  .crf-banner{margin:6px 0 8px;padding:10px 12px;border-radius:6px;font-size:13px;line-height:1.4}
  .crf-banner-warn{border:1px solid var(--crf-err);background:color-mix(in srgb,var(--crf-err) 12%,var(--background-color));color:var(--text-color)}
  .crf-banner-info{border:1px solid var(--crf-info);background:color-mix(in srgb,var(--crf-info) 10%,var(--background-color));color:var(--text-color)}
  uui-button:not(:defined),uui-brand-button:not(:defined){cursor:pointer;border:1px solid var(--border-color);border-radius:6px;padding:5px 12px;font-size:13px;color:var(--text-color)}
  .crf-toast{position:fixed;right:18px;bottom:48px;z-index:9999;background:var(--inverse-background-color,#222);color:var(--inverse-text-color,#fff);border:1px solid var(--border-color);border-radius:6px;padding:9px 16px;font-size:13px;opacity:0;transform:translateY(6px);transition:opacity .2s,transform .2s;pointer-events:none}
  .crf-toast-show{opacity:1;transform:none}
  .crf-ball{width:12px;height:12px;border-radius:50%;background:var(--disabled-text-color,#888);display:inline-block}
  .crf-ball-idle{background:var(--crf-ok)}
  /* Blue, not amber: colour means health, and amber means degraded. A runner
     executing a job is ACTIVE — the busiest fleet is not the sickest one. */
  .crf-ball-busy{background:var(--crf-info);animation:crf-pulse 1.6s ease-in-out infinite}
  .crf-ball-error{background:var(--crf-err)}
  .crf-ball-starting{animation:crf-pulse 1.1s ease-in-out infinite}
  .crf-console{border:1px solid var(--border-color);border-radius:6px;overflow:hidden;margin:0 0 8px}
  .crf-console-head{display:flex;align-items:center;justify-content:space-between;padding:3px 6px 3px 12px;min-height:30px;box-sizing:border-box;background:var(--table-header-background-color);font-size:11px;color:var(--alt-text-color)}
  .crf-lg-dim{color:var(--alt-text-color)}
  .crf-lg-ok{color:var(--crf-ok)}
  .crf-lg-warn{color:var(--crf-busy)}
  .crf-lg-err{color:var(--crf-err)}
  .crf-console-body{white-space:pre-wrap;font-family:bitstream,monospace;font-size:12px;line-height:1.5;min-height:90px;max-height:200px;overflow:auto;background:var(--shade-bg-color,var(--background-color));color:var(--text-color);padding:6px 10px}
  .crf-builder-wrap textarea{width:100%;font-family:bitstream,monospace;font-size:12px;background:var(--input-bg-color);color:var(--text-color);border:1px solid var(--textarea-border-color,var(--input-border-color))}
  /* Config form (crf_render_fields), rendered identically by the Fleet tab for the
     fleet layer and the Settings tab for the global layer. Capped at 780px on purpose:
     a settings form read left to right across a 2500px monitor is unreadable, and the
     multi-column card grid this replaced made "which section am I in" a scanning
     problem. The 222px left margin lines help + actions up under the fields. */
  .crf-cfg{max-width:780px;margin-bottom:8px}
  .crf-cfg-title{font-size:17px;font-weight:500;margin:0;color:var(--text-color)}
  .crf-cfg-head{margin-bottom:14px}
  .crf-cfg-for{display:block;font-size:12px;margin-top:3px;line-height:1.45}
  .crf-cfg-fs{border:none;margin:0;padding:0;min-width:0}
  .crf-cfg-fs[disabled]{opacity:.55}
  .crf-cfg-sec{margin:0 0 18px}
  .crf-cfg-h{font-size:13px;font-weight:500;color:var(--text-color);margin:0 0 2px;padding:0;border:none;letter-spacing:.01em}
  .crf-cfg-blurb{font-size:12px;color:var(--alt-text-color);margin:0 0 8px;line-height:1.45}
  .crf-cfg dl.crf-cfg-row{margin:0!important;padding:3px 0!important;display:flex;align-items:center;gap:12px;background:none!important}
  .crf-cfg dl.crf-cfg-row dt{float:none!important;width:210px!important;min-width:0;flex:0 0 210px;text-align:right;font-size:12px;margin:0!important;padding:0!important;background:none!important}
  .crf-cfg dl.crf-cfg-row dd{float:none!important;flex:1;min-width:0;width:auto!important;margin:0!important;padding:0!important;background:none!important}
  .crf-cfg dd input[type=text],.crf-cfg dd input[type=number],.crf-cfg dd select{width:100%!important;max-width:none!important;font-size:12px;padding:3px 6px;margin:0}
  .crf-cfg .inline_help{font-size:12px;color:var(--alt-text-color);background:var(--shade-bg-color,transparent);border-left:2px solid var(--border-color);border-radius:0;margin:2px 0 8px 222px;padding:6px 10px;line-height:1.45}
  .crf-cfg .inline_help p{margin:0}
  .crf-cfg-actions{display:flex;gap:8px;padding:6px 0 2px;margin-left:222px}
  .crf-cfg-warn{display:none;font-size:12px;color:var(--crf-err);font-weight:bold;margin:0 0 6px 222px}
  .crf-cfg-warn.on{display:block}
  .crf-cfg-note{display:none;font-size:12px;color:var(--crf-busy);margin:0 0 6px 222px}
  .crf-cfg-note.on{display:block}
  @media (max-width:860px){
    .crf-cfg dl.crf-cfg-row{display:block}
    .crf-cfg dl.crf-cfg-row dt{width:auto!important;flex:none;text-align:left}
    .crf-cfg .inline_help,.crf-cfg-actions,.crf-cfg-warn,.crf-cfg-note{margin-left:0}
  }
  @keyframes crf-pulse{0%,100%{opacity:1}50%{opacity:.35}}
  @media (prefers-reduced-motion:reduce){.crf-ball-busy,.crf-ball-starting{animation:none}}
</style>
<script>
/* Every form field's built-in default, keyed by cfg key. Backs each tab's "Reset to
   defaults" button and the value a form falls back to for any key not written to
   flash — so flash keeps holding only what the operator actually changed. */
const CRF_DEFAULTS = <?=json_encode($crf_defaults)?>;
const CRF_CSRF = "<?=$crf_csrf?>";
const CRF_URL  = "/plugins/ci-runner-farm/include/exec.php";
const CRF_UUI_BASE = "<?=$crf_uui_base?>";
const CRF_UTIL_CSS = "<?=$crf_util_css?>";
function crfDark(){ return /Theme--(black|gray)\b/.test(document.documentElement.className); }
/* Force-register @unraid/ui (see header comment). Resolves both hashed asset
   names at runtime; merges the standalone Tailwind utilities (which the uui
   bundle ships without) with the uui tokens, rescoped for shadow DOM. */
window.CRF_UUI = (async () => {
  try {
    // Fetch the manifest and the standalone utility CSS concurrently: the util CSS URL is
    // resolved server-side (PHP glob) and doesn't depend on the manifest, so there's no
    // reason to await one before starting the other. Cuts a round-trip off first paint.
    const [man, utilCss] = await Promise.all([
      fetch(CRF_UUI_BASE + 'ui.manifest.json').then(r => r.json()),
      CRF_UTIL_CSS ? fetch(CRF_UTIL_CSS).then(r => r.text()) : Promise.resolve('')
    ]);
    // Distinguish a manifest SHAPE change (bundle updated, entries renamed) from an
    // absent bundle (fetch throws -> outer catch), so a future @unraid/ui update logs
    // an actionable message rather than a generic "unavailable".
    if (!(man['style.css'] && man['style.css'].file && man['src/register.ts'] && man['src/register.ts'].file)) {
      console.warn('ci-runner-farm: @unraid/ui manifest shape changed (style.css/register.ts entries missing) — bundle updated; using fallback. Update the crf-core.php loader.');
      return false;
    }
    // The uui stylesheet and the register.ts module are independent — fetch the CSS and
    // import the module concurrently, then merge + rescope for shadow DOM and register.
    const [uuiCss, mod] = await Promise.all([
      fetch(CRF_UUI_BASE + man['style.css'].file).then(r => r.text()),
      import(CRF_UUI_BASE + man['src/register.ts'].file)
    ]);
    const css = [utilCss, uuiCss].filter(Boolean).join('\n')
      .replace(/\.unapi\.dark\b/g, ':host(.dark)')
      .replace(/\.unapi\b/g, ':host')
      .replace(/:root\b/g, ':host')
      .replace(/\.dark\b/g, ':host(.dark)')
      + '\n:host([size="xs"]) .inline-flex{font-size:12px;line-height:1.2;padding:3px 10px;gap:4px}'
      + '\n:host(.crf-stat) [class~="p-4"]{padding:8px 14px}';
    mod.registerAllComponents({ sharedCssContent: css });
    if (crfDark()) document.querySelectorAll('uui-button,uui-brand-button,uui-badge,uui-card-wrapper').forEach(e => e.classList.add('dark'));
    return true;
  } catch (e) { console.warn('ci-runner-farm: @unraid/ui unavailable, using fallback styling', e); return false; }
})();
/* Which fleet every request on this page is about.
   One install can host several named fleets; exec.php takes a `fleet` param on
   every action and falls back to `default` when it is absent. That fallback is
   why the selection is injected HERE rather than threaded through each call
   site: a single missed call would silently operate on the wrong fleet, and
   stopping the wrong fleet has no visible symptom until someone notices their
   runners are gone. One choke point, no per-call-site discipline required.
   Persisted per browser (not on flash) — it is a view preference, not config. */
const CRF_FLEET_KEY = 'crf.fleet';
window.CRF_FLEET = (() => { try { return localStorage.getItem(CRF_FLEET_KEY) || 'default'; } catch(e) { return 'default'; } })();
function crfFleet(){ return window.CRF_FLEET || 'default'; }
function crfSetFleet(name){
  window.CRF_FLEET = name || 'default';
  try { localStorage.setItem(CRF_FLEET_KEY, window.CRF_FLEET); } catch(e) {}
  document.dispatchEvent(new CustomEvent('crf-fleet-change', {detail:{fleet:window.CRF_FLEET}}));
}
function crfPost(p){
  p.csrf_token = CRF_CSRF;
  // Explicit fleet wins (fleets-json and the fleet verbs name their own target).
  if(p.fleet===undefined) p.fleet = crfFleet();
  return fetch(CRF_URL,{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},
    body:Object.entries(p).map(([k,v])=>encodeURIComponent(k)+'='+encodeURIComponent(v)).join('&')})
    .then(r=>r.text().then(t=>{
      // fetch() only rejects on network failure, not on 4xx/5xx — so an expired
      // CSRF token (403 after a reboot/array restart) or a backend 500 arrives
      // here with a JSON body that would otherwise parse and resolve as if it were
      // real data, making the fleet look empty and buttons silently no-op. Reject
      // instead, so callers' .catch paints "connection lost / reload".
      if(!r.ok) throw new Error('http '+r.status+(r.status===403?' — session expired, reload the page':'')+': '+t.slice(0,100));
      let d; try{ d=JSON.parse(t); }catch(e){ throw new Error('bad response for '+p.action+': '+t.slice(0,120)); }
      // A request answered for one fleet that has since moved: a switch can land in
      // the ~5s the status poll takes, and the response (carrying the previous
      // fleet's data) would otherwise paint under the new fleet's name for one
      // cycle. exec.php stamps `fleet` on every poll response that has one — if the
      // stamp disagrees with what was asked, the response is stale; drop it by
      // rejecting with a tag callers' .catch can ignore. User-initiated mutating
      // actions (start/stop/validate) don't echo a fleet and are not subject to
      // the race because the click is on the same tab.
      if(d&&typeof d==='object'&&typeof d.fleet==='string'&&d.fleet!==p.fleet){
        const e=new Error('stale'); e.__crfStale=true; throw e;
      }
      return d;
    }));
}
/* Copy text to the clipboard, feature-detecting the async Clipboard API (absent
   in insecure contexts — Unraid's LAN webGUI is often plain HTTP, where
   navigator.clipboard is undefined and .writeText would throw synchronously) and
   falling back to execCommand('copy'). Shared by every tab (one document). */
function crfCopyText(t){
  const legacy=()=>new Promise((res,rej)=>{ try{ const ta=document.createElement('textarea'); ta.value=t; ta.setAttribute('readonly',''); ta.style.position='fixed'; ta.style.top='-1000px'; ta.style.opacity='0'; document.body.appendChild(ta); ta.select(); const ok=document.execCommand('copy'); document.body.removeChild(ta); ok?res():rej(new Error('copy rejected')); }catch(e){ rej(e); } });
  // Some browsers EXPOSE navigator.clipboard on a plain-HTTP LAN page but then REJECT
  // writeText (insecure/unfocused context) — so .catch the promise and fall through to
  // the execCommand textarea, rather than surfacing the rejection as "Copy failed".
  if(navigator.clipboard&&navigator.clipboard.writeText) return navigator.clipboard.writeText(t).catch(legacy);
  return legacy();
}
/* Copy the text content of an element to the clipboard, with button feedback. */
function crfCopyFrom(id, btn){
  const t=(document.getElementById(id)||{}).textContent||'';
  crfCopyText(t).then(()=>{ const o=btn.textContent; btn.textContent='Copied'; setTimeout(()=>btn.textContent=o,1500); }).catch(()=>{ const o=btn.textContent; btn.textContent='Copy failed'; setTimeout(()=>btn.textContent=o,1500); });
}
function crfEsc(s){ return String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])); }
/* Semantic log tinting: escape first, then dim structural prefixes and tint
   error/warn lines and known lifecycle phrases. Applies to both consoles. */
function crfColorize(t){
  return String(t||'').split('\n').map(line=>{
    let l=crfEsc(line);
    l=l.replace(/^(\[ci-runner-farm\]|\d{4}-\d{2}-\d{2}[T ][\d:.]+Z?|#\d+\s)/,'<span class="crf-lg-dim">$1</span>');
    if(/error|fatal|failed|failure/i.test(line)) return '<span class="crf-lg-err">'+l+'</span>';
    if(/warn/i.test(line)) return '<span class="crf-lg-warn">'+l+'</span>';
    l=l.replace(/\b(shrink by \d+|removing idle [\w-]+|deregistered [\w-]+|reaping [\w-]+|stopping|stopped)\b/gi,'<span class="crf-lg-warn">$1</span>');
    l=l.replace(/\b(grow to \d+|daemon up|started|registered|Listening for Jobs|successfully|DONE|CACHED|FINISHED|build complete)\b/gi,'<span class="crf-lg-ok">$1</span>');
    return l;
  }).join('\n');
}
function crfToast(msg){
  let t=document.getElementById('crf-toast');
  if(!t){ t=document.createElement('div'); t.id='crf-toast'; t.className='crf-toast'; document.body.appendChild(t); }
  t.textContent=msg; t.classList.add('crf-toast-show');
  clearTimeout(t._h); t._h=setTimeout(()=>t.classList.remove('crf-toast-show'),2600);
}
</script>
