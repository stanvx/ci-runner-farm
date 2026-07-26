<?php
/* CI Runner Farm — the form field table and its renderer.
   One data table per layer instead of 27 hand-written blocks: the Fleet tab renders
   `fleet`, the Settings tab renders `global`, and both get the same markup, the same
   inline-help behaviour and the same "which keys does this form own" answer.

   Kept OUT of crf-config.php on purpose: config-parity.sh reads the $crf_defaults
   array literal, and the 'key'=>'string' pairs below would otherwise read as defaults.

   Emits the same DOM Unraid's markdown forms produce — <dl><dt>/<dd> rows followed by
   <blockquote class="inline_help"> — so the webGUI's help toggle and every theme's
   form styling apply without a per-theme override. Written as plain HTML rather than
   markdown because these forms are POSTed by fetch (see save-config), not by
   /update.php, so there is no `#file` to hardcode and nothing to re-grid afterwards. */

/* [key, label, type, opts]. type: text | number | select.
   opts: placeholder, min, max, id, onchange, options (value=>label), help (HTML). */
function crf_fields($layer) {
  $fleet = [
    ['GitHub access', 'How this fleet registers with GitHub.', [
      ['GH_SCOPE','GitHub scope','select',['options'=>[
          'repo'=>'repo (per-repository)','org'=>'org (organization-wide)'],
        'help'=>'repo = register runners on specific repositories.<br><br>org = one shared pool available to every repo in the org (recommended when you have more than one repo).']],
      ['GH_OWNER','Org owner','text',['help'=>'GitHub organization name. Used only when scope is org (e.g. unraid).']],
      ['GH_REPOS','Target repos','text',['help'=>'Space-separated owner/repo list, used when scope is repo. Runners are spread across these repos round-robin.']],
      ['RUNNER_GROUP','Runner group','text',['help'=>'Optional runner group (org scope) to restrict which repos may use the pool &mdash; e.g. exclude public repos so fork PRs never run here.']],
      ['RUNNER_LABELS','Runner labels','text',['help'=>'Comma-separated labels that workflows target with runs-on, e.g. self-hosted,unraid,build.']],
    ]],
    ['Capacity', 'How many runners this fleet keeps, and whether that number is fixed.', [
      ['RUNNER_COUNT','Concurrent runners','number',['min'=>1,'max'=>20,
        'help'=>'How many runner containers to launch. Each runs one job at a time, so this equals your maximum number of concurrent builds.']],
      ['AUTOSCALE','Autoscaling','select',['options'=>[
          'false'=>'off (fixed at Concurrent runners)','true'=>'on (fleet floats by demand)'],
        'help'=>'When on, a daemon grows/shrinks the fleet by demand instead of using a fixed count. It keeps a warm idle buffer: when idle runners drop below the buffer (jobs are consuming them) it adds runners; when too many sit idle it removes them. Bounded by Min/Max, with grace to avoid flapping. Only ever removes idle runners, never a runner mid-job.']],
      ['AUTOSCALE_MIN','Autoscale: min runners','number',['min'=>0,'max'=>40,
        'help'=>'The floor &mdash; the fleet never drops below this, even fully idle. Keeps a couple warm so the first jobs never wait.']],
      ['AUTOSCALE_MAX','Autoscale: max runners','number',['min'=>1,'max'=>40,
        'help'=>'The ceiling &mdash; the fleet never grows past this. Set it to the most the box can run alongside the other workloads (~16 leaves CPU/RAM headroom on a typical host).']],
      ['AUTOSCALE_MIN_IDLE','Autoscale: warm idle buffer','number',['min'=>1,'max'=>20,
        'help'=>'Keep at least this many idle runners ready. When idle drops below it, scale up &mdash; so a burst of jobs finds warm runners instead of queueing. Higher = more headroom but more idle containers.']],
      ['AUTOSCALE_STEP','Autoscale: step','number',['min'=>1,'max'=>10,
        'help'=>'How many runners to add or remove per adjustment.']],
      ['AUTOSCALE_INTERVAL','Autoscale: check interval (s)','number',['min'=>10,'max'=>600,
        'help'=>'Seconds between demand checks. Lower = more responsive, more churn.']],
      ['AUTOSCALE_IDLE_GRACE','Autoscale: scale-down grace','number',['min'=>1,'max'=>60,
        'help'=>'How many consecutive over-idle checks before removing runners. Scale-up is immediate; scale-down waits this long to avoid flapping during brief lulls.']],
    ]],
    ['Execution', 'What a job runs on, and what it runs as.', [
      ['IMAGE_SOURCE','Image source','select',['onchange'=>'crfCfgImgSrc()','options'=>[
          'builtin'=>'Built-in (build locally)','remote'=>'Remote registry image'],
        'help'=>'<b>Built-in</b> (default): run the image you build on the Runner image tab &mdash; no registry needed. <b>Remote</b>: pull the image named below from a registry (its server and username are host-wide, on the Settings tab).']],
      ['IMAGE','Remote image','text',['placeholder'=>'ghcr.io/org/image:tag',
        'help'=>'Used only when Image source = <b>Remote</b>. Full image ref to pull, e.g. <code>ghcr.io/org/ci-runner-image:latest</code>. For a private image, set the registry server, username and token on the Settings tab.']],
      ['EPHEMERAL','Ephemeral','select',['options'=>[
          'false'=>'false (persistent, warm cache)','true'=>'true (clean per job)'],
        'help'=>'false = persistent runner, package caches stay warm between jobs. true = fresh runner re-registered per job (cleanest state, slightly slower first run).']],
      ['RUN_AS_ROOT','Run jobs as root','select',['options'=>[
          'false'=>"false (non-root 'runner', like GitHub-hosted)",'true'=>'true (root, legacy)'],
        'help'=>'false (recommended): jobs run as the non-root <code>runner</code> user with passwordless sudo, matching GitHub-hosted runners &mdash; so tests that assert non-root file permissions behave correctly. Package caches mount under /home/runner for that user. true: jobs run as root (legacy); permission-sensitive tests may behave differently and caches mounted under /home/runner won&rsquo;t be auto-discovered by root tooling.']],
    ]],
    ['Resources', 'The per-runner ceiling on this box.', [
      ['RUNNER_CPUS','CPUs per runner','text',['placeholder'=>'blank = uncapped',
        'help'=>'Hard CPU cap per runner. Leave blank for uncapped &mdash; the Linux scheduler time-shares CPU fairly across all runners, so a lone build can use the whole box.']],
      ['RUNNER_MEMORY','Memory per runner','text',['help'=>'Hard memory cap per runner, e.g. 16g. Kept capped because memory is not time-shared like CPU &mdash; an uncapped leak could OOM the host or the other workloads.']],
      ['WORK_TMPFS_SIZE','Workspace tmpfs size','text',['placeholder'=>'blank = bind to pool',
        'help'=>'Size of the RAM-backed per-job workspace, e.g. 8g &mdash; a clean, fast workspace each job while caches stay warm. Blank binds the workspace to the pool instead of RAM.']],
    ]],
    ['Caching', 'Warm caches mounted into every runner of this fleet. The pool path they live under is host-wide, on the Settings tab.', [
      ['CACHE_MOUNTS','Cache mounts','text',['help'=>'Space-separated <code>host-subdir:container-path</code> warm caches mounted into every runner (created under the host-wide cache root). Defaults cover pnpm/npm/yarn/Playwright &mdash; edit for your stack.']],
    ]],
    ['Security', 'What a job on this fleet can reach. Read the posture block above first &mdash; it reports what the engine actually observed.', [
      ['DIND','Docker-in-docker mode','select',['options'=>[
          'true'=>'true (own daemon per runner, privileged)','false'=>'false (use host docker.sock)'],
        'help'=>'Docker-in-docker: each runner runs its own Docker daemon (requires --privileged).<br><br>Fixes GitHub Actions services: networking (localhost:port becomes reachable from job steps) and "port already allocated" collisions between parallel jobs. Recommended when workflows use services: or service containers.<br><br>Each runner&rsquo;s <code>/var/lib/docker</code> is stored under the host-wide <b>Cache root</b>, so when this is on, Cache root <b>must</b> be a real pool dataset (not a <code>/mnt/user</code> FUSE share) or <code>docker build</code>/<code>buildx</code>/<code>services:</code> will fail with overlay mount errors.']],
      ['SHARE_DOCKER_SOCK','Share host docker.sock','select',['options'=>['false'=>'false','true'=>'true'],
        'help'=>'Mount the host Docker socket so jobs can start service containers (e.g. postgres for integration tests). PRIVATE repos only &mdash; never expose this to public/fork-PR code (root-equivalent host access).']],
      ['NETWORK_ISOLATION','Network isolation','select',['options'=>[
          'off'=>'off (default docker bridge)','isolate'=>'isolate (dedicated bridge)','strict'=>'strict (dedicated bridge + block host/LAN)'],
        'help'=>'Confines the runners at the network layer. Applies on the next <b>Start</b>.<br><br><b>off</b> &mdash; runners share the default Docker bridge (legacy behavior).<br><br><b>isolate</b> &mdash; runners run on a dedicated bridge, so they can&rsquo;t reach your <b>other Unraid containers</b>. Each fleet gets its own bridge, and the shared image cache joins it. Low-risk; recommended.<br><br><b>strict</b> &mdash; everything <b>isolate</b> does, <b>plus</b> IPv4 firewall rules (Docker&rsquo;s <code>DOCKER-USER</code>/<code>INPUT</code> chains) that block runners from reaching the <b>Unraid host and your LAN</b> while still allowing the internet and the shared cache. Treat this as <b>defense-in-depth against accidental egress</b> (a build script phoning home by mistake), <b>not a hard boundary</b>: with Docker-in-Docker on (the default) runners are <b>privileged</b> and determined malicious code could break out and remove these rules, and the rules are IPv4-only. For genuinely untrusted code, prefer non-privileged runners and treat strict as an extra layer, not the wall. Requires <code>iptables</code>; if it can&rsquo;t be applied, Start logs a warning and continues without the egress rules.']],
    ]],
    ['Advanced', 'Image auto-update for this fleet.', [
      ['IMAGE_AUTOUPDATE','Auto-update runner image','select',['options'=>[
          'false'=>'off (update manually)','true'=>'on (pull on a schedule, roll the fleet)'],
        'help'=>'When on, a daemon checks the runner image on a schedule and, when a newer image is published, recreates each runner on it &mdash; <b>draining</b> first (waits for the runner&rsquo;s current job to finish, never interrupts a build). The shared image-cache mirror is refreshed in the same pass.<br><br>Only <b>remote</b> images (Image source = Remote) can auto-pull &mdash; a built-in image has no upstream, so rebuild it from the Runner image tab instead.<br><br>Takes effect on the next <b>Start/Restart</b> of the fleet (like Autoscaling).']],
      ['IMAGE_AUTOUPDATE_INTERVAL','Auto-update: check interval (s)','number',['min'=>300,'max'=>86400,
        'help'=>'Seconds between update checks. Default 1800 (30 min). A check is a cheap registry digest comparison; the fleet only rolls when the image actually changed.']],
      ['IMAGE_DRAIN_TIMEOUT','Auto-update: drain timeout (s)','number',['min'=>0,'max'=>86400,
        'help'=>'How long to wait for a busy runner to finish its current job before recreating it on the new image. If it&rsquo;s still busy after this, it&rsquo;s left on the old image and retried next cycle. <code>0</code> = wait forever (never recreate a busy runner). A runner is only ever recreated while idle.']],
    ]],
  ];
  $global = [
    ['Storage', 'One pool path for the whole box. Every fleet&rsquo;s caches, Docker data roots and bind workspaces live under it.', [
      ['CACHE_ROOT','Cache root','text',['id'=>'CACHE_ROOT','placeholder'=>'/mnt/cache/github-runner',
        'attrs'=>'autocomplete="off" spellcheck="false" data-pickroot="/mnt" data-pickfolders="true" data-pickfilter="HIDE_FILES_FILTER"',
        'help'=>'Pool path for warm shared caches (pnpm/npm/yarn), each Docker-in-Docker runner&rsquo;s Docker data root, and (if tmpfs is off) the bind workspaces. Click the field to browse and pick a folder.<br><br><b>Use a dedicated subdirectory of a pool dataset (e.g. <code>/mnt/&lt;pool&gt;/github-runner</code>), not a bare pool/disk root and not a <code>/mnt/user/...</code> user share.</b> A bare mount root (<code>/mnt/cache</code>, <code>/mnt/disk1</code>) is rejected because clearing caches deletes under this path &mdash; it must not sit on top of your appdata, VMs, or other shares. User shares are FUSE (fuse.shfs), and with Docker-in-Docker on, overlay2 cannot run on FUSE &mdash; so <code>buildx</code> and <code>services:</code> jobs fail with <code>mount overlay ... invalid argument</code>. Each fleet&rsquo;s status warns you if this path is unsafe.<br><br>A <code>/mnt/user/&lt;name&gt;</code> path <b>is</b> fine when <code>&lt;name&gt;</code> is a symlink (or share) that resolves to a dedicated pool dataset rather than a FUSE share &mdash; the path is canonicalized before it is checked, so it is validated (and caches/DinD land) at the real pool location.']],
      ['SHARED_IMAGE_CACHE','Shared image cache','select',['options'=>[
          'true'=>'true (pull-through registry mirror)','false'=>'false'],
        'help'=>'Runs one shared <code>registry:2</code> pull-through cache (Docker-in-Docker only) so images used across every fleet are pulled from Docker Hub once, not once per runner. Bound to the Docker bridge gateway, not the LAN &mdash; so it is not exposed off the host, though (being on the bridge gateway) it is reachable unauthenticated by any container on the default Docker bridge, not only this plugin&rsquo;s runners. It only caches public Docker Hub images. Turn off if you don&rsquo;t want the extra container. The host port defaults to 5000; if that clashes with another service, set <code>MIRROR_PORT</code> in <code>/boot/config/plugins/ci-runner-farm/ci-runner-farm.cfg</code> and Restart.']],
    ]],
    ['Private registry', 'One host-wide <code>docker login</code>. The password/token is saved above, in a chmod-600 file, never in the cfg.', [
      ['REGISTRY_SERVER','Registry server','text',['placeholder'=>'blank = none, e.g. ghcr.io',
        'help'=>'Private registry to <code>docker login</code> so the host can pull a private runner image (each fleet&rsquo;s Remote image, on the Fleet tab). Blank disables it. For <code>ghcr.io</code>, if you leave the registry token blank the GitHub PAT is reused automatically (the PAT must have the <code>read:packages</code> scope), and the username defaults to the org/owner.']],
      ['REGISTRY_USERNAME','Registry username','text',['help'=>'Username for the private registry (e.g. your GitHub username for ghcr.io).']],
    ]],
    ['Dashboard', 'The Unraid Main &rarr; Dashboard tile.', [
      ['DASHBOARD_WIDGET_ENABLE','Main Dashboard tile','select',['options'=>[
          'true'=>'true (show the tile)','false'=>'false'],
        'help'=>'Show the CI Runner Farm status tile on the Unraid <b>Main &rarr; Dashboard</b> (up/busy/idle counts, pushed live). The tile broadcasts only box-wide aggregate counts &mdash; never a fleet name, repo, branch or job. Turn off to hide it.']],
    ]],
  ];
  return $layer === 'global' ? $global : $fleet;
}

/* Every key one layer's form owns — what save-config is handed and what "Reset to
   defaults" walks. Derived from the table so adding a field cannot forget either. */
function crf_field_keys($layer) {
  $out = [];
  foreach (crf_fields($layer) as $s) foreach ($s[2] as $f) $out[] = $f[0];
  return $out;
}

/* Render one layer's sections. $cfg supplies stored values; anything absent falls back
   to $crf_defaults via crf_g/crf_sel. */
function crf_render_fields($layer, $cfg) {
  foreach (crf_fields($layer) as [$title, $blurb, $rows]) {
    $anchor = 'crf-cfg-' . strtolower(preg_replace('/[^a-z]+/i', '-', $title));
    echo '<section class="crf-cfg-sec" id="' . $anchor . '">';
    echo '<h3 class="crf-cfg-h">' . $title . '</h3>';
    if ($blurb !== '') echo '<p class="crf-cfg-blurb">' . $blurb . '</p>';
    foreach ($rows as [$k, $label, $type, $o]) {
      $o += ['placeholder'=>'', 'min'=>null, 'max'=>null, 'id'=>'', 'onchange'=>'', 'attrs'=>'', 'help'=>'', 'options'=>[]];
      $attrs = ($o['id'] ? ' id="' . $o['id'] . '"' : '') . ' name="' . $k . '"'
        . ($o['onchange'] ? ' onchange="' . $o['onchange'] . '"' : '')
        . ($o['attrs'] ? ' ' . $o['attrs'] : '');
      echo '<dl class="crf-cfg-row" data-key="' . $k . '"><dt>' . $label . ':</dt><dd>';
      if ($type === 'select') {
        echo '<select' . $attrs . '>';
        foreach ($o['options'] as $v => $l) {
          echo '<option value="' . htmlspecialchars($v, ENT_QUOTES) . '" ' . crf_sel($cfg, $k, (string)$v) . '>' . htmlspecialchars($l) . '</option>';
        }
        echo '</select>';
      } else {
        echo '<input type="' . ($type === 'number' ? 'number' : 'text') . '"' . $attrs
          . ($o['min'] !== null ? ' min="' . $o['min'] . '"' : '')
          . ($o['max'] !== null ? ' max="' . $o['max'] . '"' : '')
          . ($o['placeholder'] !== '' ? ' placeholder="' . htmlspecialchars($o['placeholder'], ENT_QUOTES) . '"' : '')
          . ' value="' . crf_g($cfg, $k) . '">';
      }
      echo '</dd></dl>';
      if ($o['help'] !== '') echo '<blockquote class="inline_help"><p>' . $o['help'] . '</p></blockquote>';
    }
    echo '</section>';
  }
}
