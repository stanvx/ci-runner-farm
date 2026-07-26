<?php
require __DIR__ . '/../src/usr/local/emhttp/plugins/ci-runner-farm/include/crf-config.php';

$file = tempnam(sys_get_temp_dir(), 'crf-config-');
$lock = $file . '.lock';

function check_value($file, $key, $expected) {
  $cfg = parse_ini_file($file, false, INI_SCANNER_RAW);
  if (!is_array($cfg) || ($cfg[$key] ?? null) !== $expected) {
    throw new RuntimeException("$key did not persist as expected");
  }
}

try {
  file_put_contents($file, "GH_REPOS=\"old/repo\"\nCACHE_ROOT=\"/mnt/old\"\n");

  // Global then fleet: the fleet save must not erase CACHE_ROOT.
  if (!crf_cfg_merge($file, ['CACHE_ROOT' => '/mnt/cache_nvme/ci-runner-farm'], $lock)) throw new RuntimeException('global save failed');
  if (!crf_cfg_merge($file, ['GH_REPOS' => 'stanvx/scribekey'], $lock)) throw new RuntimeException('fleet save failed');
  check_value($file, 'CACHE_ROOT', '/mnt/cache_nvme/ci-runner-farm');
  check_value($file, 'GH_REPOS', 'stanvx/scribekey');

  // Fleet then global: the reverse order must preserve both layers too.
  if (!crf_cfg_merge($file, ['GH_REPOS' => 'stanvx/other'], $lock)) throw new RuntimeException('fleet save failed');
  if (!crf_cfg_merge($file, ['CACHE_ROOT' => '/mnt/cache_nvme/ci-runner-farm-2'], $lock)) throw new RuntimeException('global save failed');
  check_value($file, 'CACHE_ROOT', '/mnt/cache_nvme/ci-runner-farm-2');
  check_value($file, 'GH_REPOS', 'stanvx/other');
  echo "config-merge: OK\n";
} finally {
  @unlink($file);
  @unlink($lock);
}
