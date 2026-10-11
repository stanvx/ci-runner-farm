<?php
// Exercise the endpoint's own command/JSON boundary without Unraid's CSRF files.
$src = file_get_contents(__DIR__ . '/../src/usr/local/emhttp/plugins/ci-runner-farm/include/exec.php');
$start = strpos($src, 'function crf_secret_values(');
if ($start === false) throw new RuntimeException('response redaction is missing');
$end = strpos($src, 'switch ($action)', $start);
eval(substr($src, $start, $end - $start));

$CFGDIR = sys_get_temp_dir() . '/crf-redaction-' . bin2hex(random_bytes(6));
mkdir($CFGDIR . '/credentials', 0700, true);
$registry = 'registry/secret"with\\escapes';
$pat = 'github_pat_namedCredential123456789';
$key = "-----BEGIN PRIVATE KEY-----\nprivate-test-key-material\n-----END PRIVATE KEY-----";
file_put_contents("$CFGDIR/registry-token", $registry);
file_put_contents("$CFGDIR/credentials/named.token", $pat);
file_put_contents("$CFGDIR/credentials/named.app.pem", $key);
function check($ok, $message) { if (!$ok) throw new RuntimeException($message); }
try {
  $body = ['log' => "$registry $pat $key", 'nested' => ['token' => 'ghp_unknownToken123456789'], 'bytes' => "bad\xff"];
  $encoded = crf_json($body);
  $decoded = json_decode($encoded, true, 512, JSON_THROW_ON_ERROR);
  check(!str_contains($decoded['log'], $registry), 'escaped registry secret leaked through JSON');
  check(!str_contains($decoded['log'], $pat), 'named PAT leaked');
  check(!str_contains($decoded['log'], 'private-test-key-material'), 'app key leaked');
  check(!str_contains($decoded['nested']['token'], 'unknownToken'), 'token-shaped value leaked');
  [$plain, $rc] = run('printf %s ' . escapeshellarg($registry));
  check($rc === 0 && $plain === '[REDACTED]', 'command log boundary leaked');
  [$json, $rc] = run_json('printf %s ' . escapeshellarg(json_encode($body, JSON_INVALID_UTF8_SUBSTITUTE)));
  check($rc === 0 && !str_contains(json_decode($json, true)['log'], $registry), 'raw JSON command leaked escaped secret');
  $last = last_json("progress\n" . json_encode(['error' => $registry]));
  check(json_decode($last, true)['error'] === '[REDACTED]', 'last-line JSON verdict leaked escaped secret');
  echo "upstream-safety: response redaction OK\n";
} finally {
  foreach (glob("$CFGDIR/credentials/*") as $file) unlink($file);
  rmdir("$CFGDIR/credentials");
  unlink("$CFGDIR/registry-token");
  rmdir($CFGDIR);
}
