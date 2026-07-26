<?php
/* CI Runner Farm — the config layer shared by the pages and exec.php.
   Holds the one PHP copy of the built-in defaults, resolves which cfg file a
   (fleet, layer) pair writes to, and merges values into it.

   Config is TWO fixed layers, exactly as the engine assembles them (runner-farm.sh
   GLOBAL_KEYS / FLEET_KEYS): `global` describes the box and always lives in the legacy
   cfg; `fleet` describes one fleet and lives in that fleet's own cfg. There is no
   inheritance and no sparse-override model — a key belongs to exactly one layer.

   The key LISTS are parsed out of the engine rather than restated here. A fourth
   hand-written copy of them is a drift source config-parity.sh would then have to
   police; parsing means the engine stays the single authority. */

const CRF_PLUGIN  = 'ci-runner-farm';
const CRF_CFGDIR  = '/boot/config/plugins/' . CRF_PLUGIN;
const CRF_ENGINE  = '/usr/local/emhttp/plugins/' . CRF_PLUGIN . '/include/runner-farm.sh';

/* Single source of truth for every form field's default, on the PHP side. Drives the
   rendered fallback (crf_g/crf_sel), the client-side "Reset to defaults" button
   (emitted as CRF_DEFAULTS) and the values the Fleet form falls back to for any key a
   fleet cfg does not set — so those three can never drift apart.
   MUST agree with the engine's `# ---- defaults` block and default.cfg;
   tests/config-parity.sh is the gate. */
$crf_defaults = [
  'GH_SCOPE'=>'repo', 'GH_OWNER'=>'unraid', 'GH_REPOS'=>'unraid/repo-a unraid/repo-b',
  'RUNNER_GROUP'=>'', 'RUNNER_COUNT'=>'4', 'RUNNER_LABELS'=>'self-hosted,unraid,build',
  'RUNNER_CPUS'=>'', 'RUNNER_MEMORY'=>'16g', 'EPHEMERAL'=>'false', 'RUN_AS_ROOT'=>'false',
  'IMAGE_SOURCE'=>'builtin', 'IMAGE'=>'', 'REGISTRY_SERVER'=>'', 'REGISTRY_USERNAME'=>'',
  'CACHE_ROOT'=>'/mnt/cache/github-runner', 'WORK_TMPFS_SIZE'=>'8g',
  'CACHE_MOUNTS'=>'pnpm-store:/home/runner/.local/share/pnpm/store npm:/home/runner/.npm yarn:/home/runner/.cache/yarn ms-playwright:/home/runner/.cache/ms-playwright',
  'DIND'=>'true', 'SHARE_DOCKER_SOCK'=>'false', 'SHARED_IMAGE_CACHE'=>'true', 'NETWORK_ISOLATION'=>'off',
  'IMAGE_AUTOUPDATE'=>'false', 'IMAGE_AUTOUPDATE_INTERVAL'=>'1800', 'IMAGE_DRAIN_TIMEOUT'=>'3600',
  'DASHBOARD_WIDGET_ENABLE'=>'true',
  'AUTOSCALE'=>'false', 'AUTOSCALE_MIN'=>'2', 'AUTOSCALE_MAX'=>'16', 'AUTOSCALE_MIN_IDLE'=>'2',
  'AUTOSCALE_STEP'=>'2', 'AUTOSCALE_INTERVAL'=>'30', 'AUTOSCALE_IDLE_GRACE'=>'5',
];

/* Same fleet-name shape the engine and exec.php enforce. Restated as a constant so a
   caller that only include_once's this file still validates before touching a path. */
const CRF_FLEET_RE = '/^[a-z][a-z0-9-]{0,30}$/';

/* The allowlist for one layer, read from the engine's own GLOBAL_KEYS / FLEET_KEYS.
   Both are one double-quoted assignment carrying backslash line-continuations. */
function crf_layer_keys($layer) {
  static $cache = [];
  if (isset($cache[$layer])) return $cache[$layer];
  $name = $layer === 'global' ? 'GLOBAL_KEYS' : 'FLEET_KEYS';
  $src  = @file_get_contents(CRF_ENGINE) ?: '';
  if (!preg_match('/^' . $name . '="([^"]*)"/m', $src, $m)) return $cache[$layer] = [];
  return $cache[$layer] = preg_split('/\s+/', trim(str_replace('\\', ' ', $m[1])), -1, PREG_SPLIT_NO_EMPTY);
}

/* Which file a (fleet, layer) pair reads and writes.
   The global layer is ALWAYS the legacy cfg, whatever fleet is selected — one mirror,
   one cache root, one host `docker login`. Fleet `default`'s own layer is that same
   file, which is why an upgrade from single-fleet writes nothing. */
function crf_cfg_file($fleet, $layer) {
  if ($layer === 'global' || $fleet === 'default') return CRF_CFGDIR . '/' . CRF_PLUGIN . '.cfg';
  return CRF_CFGDIR . '/fleets/' . $fleet . '.cfg';
}

/* One layer's stored values — only keys actually written to flash, never defaults.
   The caller fills the gaps from $crf_defaults, so flash keeps holding just what the
   operator changed. */
function crf_cfg_read($fleet, $layer) {
  $file = crf_cfg_file($fleet, $layer);
  $cur  = is_file($file) ? (@parse_ini_file($file) ?: []) : [];
  return array_intersect_key($cur, array_flip(crf_layer_keys($layer)));
}

/* Rewrite only the given keys, in place, leaving every other line of the file alone.
   MERGE, not replace: fleet `default`'s cfg holds BOTH layers, so a global save that
   rebuilt the file from its own fields would silently delete every per-fleet key (and
   vice versa). Comments, ordering and unknown keys survive untouched. */
function crf_cfg_merge($file, array $kv) {
  $lines = is_file($file) ? file($file, FILE_IGNORE_NEW_LINES) : [];
  $seen  = [];
  foreach ($lines as $i => $l) {
    if (!preg_match('/^([A-Za-z_][A-Za-z0-9_]*)=/', $l, $m)) continue;
    if (!array_key_exists($m[1], $kv)) continue;
    $lines[$i] = $m[1] . '="' . $kv[$m[1]] . '"';
    $seen[$m[1]] = true;
  }
  foreach ($kv as $k => $v) if (empty($seen[$k])) $lines[] = $k . '="' . $v . '"';
  @mkdir(dirname($file), 0755, true);
  return file_put_contents($file, implode("\n", $lines) . "\n") !== false;
}

/* A value is written as KEY="value" and read back by a parser that strips one pair of
   surrounding quotes and splits on the FIRST '=' (runner-farm.sh load_cfg_file). So a
   raw quote or newline in a value would truncate it or forge a second key on the next
   line. Nothing this form collects legitimately contains either. */
function crf_cfg_clean($v) {
  return trim(str_replace(['"', "\r", "\n"], '', (string)$v));
}

/* Field value / selected-option helpers, backed by $crf_defaults so a key with no
   stored value renders its built-in default rather than an empty box. */
if (!function_exists('crf_g'))   { function crf_g($cfg,$k,$d=''){ return htmlspecialchars($cfg[$k] ?? $GLOBALS['crf_defaults'][$k] ?? $d, ENT_QUOTES); } }
if (!function_exists('crf_sel')) { function crf_sel($cfg,$k,$val,$d=''){ return (($cfg[$k] ?? $GLOBALS['crf_defaults'][$k] ?? $d) === $val) ? 'selected' : ''; } }
