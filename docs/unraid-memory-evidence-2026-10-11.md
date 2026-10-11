# Unraid memory and storage evidence — 11 October 2026

Host: `TRENT-UNRAID` / `192.168.1.122`; observations use AEDT. This audit separates host capacity, container limits, compressed swap, and persistent storage. Collection omitted credentials and application data contents.

## Before remediation: 17:07–17:11 AEDT

| Consumer or signal | Observed value | Meaning |
| --- | ---: | --- |
| Physical RAM | 78.31 GiB total; approximately 65 GiB used; 12.74 GiB available | A 12 GiB runner plus 2 GiB reserve cannot be admitted reliably. |
| Docker cgroup RAM | 31.81 GiB | Sum of `memory.current`; includes container file/shmem/kernel charges. |
| Docker logical swap | 10.38 GiB | Separate from physical container RAM; compressed by host zram. |
| Ubuntu VM | 12.10 GiB PSS; 0.16 GiB logical swap | Configured 12 GiB guest. Guest memory statistics are stale, so no reduction was justified. |
| Home Assistant VM | 6.16 GiB PSS; 0.41 GiB logical swap | Configured 6 GiB guest. Guest memory statistics are stale. |
| Android emulator | 11.62 GiB QEMU RSS; 1.30 GiB QEMU logical swap | Actual command overrides the 2 GiB AVD configuration with `-memory 10240`. Container uses 11.74/12 GiB and permits unlimited swap. |
| zram | 16 GiB logical swap allocated; 5.21 GiB physical allocation | Compressed RAM, not disk swap. Approximately 15.15 GiB original stored pages compress to 5.08 GiB payload. |
| Kernel slab | 4.72 GiB, including 2.93 GiB unreclaimable | Do not add ZFS ARC separately to an overlapping memory total. |
| ZFS ARC | 2.58 GiB; minimum 2.45 GiB | ARC reduction would save little at this snapshot. |

These rows overlap where stated and are not an additive accounting identity. Linux `docker stats` also subtracts inactive file cache; the cgroup accounting is the useful limit reference.

Eight seconds of `vmstat` sampling showed almost no swap-in/out and 4–7% I/O wait. Full zram indicates exhausted emergency headroom, but did not demonstrate active swap thrashing. Kernel and cgroup PSI files are unavailable on this host. zram recorded zero failed reads/writes. No `swapoff`, cache drop, VM memory reduction, or unrelated service restart was performed.

`/proc/vmstat` records 73 OOM kills since boot. Retained syslogs contain 51 `CONSTRAINT_MEMCG` OOM invocations on 9–11 October; these are container-limit failures, not proof of a host-global OOM. Java kills include 11 October at 06:30:45 and an OOM invocation at 07:18:03. A full swap gauge and Java daemon disappearance must not be treated as interchangeable diagnoses.

Largest container logical-swap charges: emulator 1.49 GiB, Plex 1.19 GiB, podly 1.15 GiB, kokoro-fastapi 0.83 GiB, dispatcharr 0.70 GiB, Frigate 0.67 GiB. These are logical sizes; their compressed physical costs cannot be attributed exactly from these counters.

## Root cause and persistent source

The emulator was a detached manual launch outside its normal supervisor, with a 10 GiB guest despite `hw.ramSize=2G`. Its supervisor's `keep_alive()` only sleeps and does not supervise replacement QEMU children. The Compose source is `/boot/config/plugins/compose.manager/projects/android-emulator/compose.yaml`.

Only `/home/androidusr/.android` was mounted persistently. Its AVD pointer references `/home/androidusr/emulator`, which was in the container writable layer. That directory contained 9.1 GiB before shutdown, including a 6.5 GiB writable userdata qcow2. Recreating this container without preserving that directory would lose the emulator's installed applications and data.

Emergency swap is configured by `/boot/config/plugins/user.scripts/scripts/codex-zram-swap/script`, scheduled at array startup in User Scripts. It creates 16 GiB zstd zram and sets `vm.swappiness=100`. Existing full zram does not drain just by reducing swappiness. Workload sizing and admission control are the primary fix; arbitrary swappiness reduction was deliberately avoided.

## Authorized remediation and verification

Backups are under `/boot/config/plugins/compose.manager/projects/android-emulator/backups/capacity-20261011-1715/`; this contains the original Compose file, exact QEMU argv, package/version proof, essential AVD file checksums, original/patched image-source diff, and image/source metadata.

The first graceful process restart used a 4 GiB guest and retained the existing AVD. It booted successfully; ScribeKey `0.9.11` / build `15` remained installed, `run-as app.scribekey pwd` confirmed its data directory, and its MainActivity launched. QEMU RSS was 4.55 GiB after application launch. The image's normal launch path reads `EMULATOR_ADDITIONAL_ARGS`, so guest RAM can be persisted there.

After a second graceful shutdown, the complete AVD was copied to `/mnt/user/appdata/android-emulator-avd`; SHA256 hashes of `config.ini`, `userdata-qemu.img`, `userdata-qemu.img.qcow2`, and `userdata.img` matched the stopped source. Its files were assigned to the image's user/group `1300:1301`. Compose now binds the real AVD path and requests a 4 GiB guest, an 8 GiB memory limit, and an 8 GiB combined memory/swap limit. ADB port `5575` and VNC port `6080` were retained. Telemetry was disabled with `USER_BEHAVIOR_ANALYTICS=false`: the image's startup telemetry POST has no timeout and can delay startup.

Docker reports `SwapLimit=false` and silently replaces the requested combined limit with `MemorySwap=-1`. Actual cgroup v2 `memory.swap.max` exists, is writable, and remained `max` until explicitly corrected. Therefore the Compose request alone does **not** enforce zero swap on this host. A root-only, exact-container guard at `/boot/config/plugins/compose.manager/projects/android-emulator/limit-swap.sh` resolves the running PID and verifies that its cgroup is exactly `/docker/<inspected-container-id>` before setting and asserting `memory.swap.max=0`. `/boot/config/plugins/compose.manager/android-emulator-swap.cron` runs it each minute; `bash /usr/local/sbin/update_cron` installed the entry. The direct cgroup value was verified as `0` with zero swap usage and zero OOM events. There is a first-minute enforcement window after a new cgroup is created. Docker caching swap capabilities before zram startup is a hypothesis, not a confirmed cause; Docker was not restarted.

Recreation exposed a separate image restart defect: `change_permission()` deletes the first `/etc/passwd` line after setting KVM ownership, removing root. A subsequent container restart cannot run its required `sudo chown`. The narrowly scoped source patch removes only that deletion and retains KVM permission handling. The source is mounted read-only and must be kept in ZFS appdata, **not** `/boot`: VFAT mounts `/boot` with `fmask=0177,dmask=0077`, so even `chmod 644` leaves files `0600` and unreadable by the image user. After explicit approval, the patch was copied to `/mnt/user/appdata/android-emulator-runtime/emulator.py` with mode 0644 and its recorded hash was verified. Compose now mounts that SSD source read-only. Fresh recreation and two subsequent graceful emulator-only container restarts passed boot, root-account, ScribeKey package/data-directory, and MainActivity launch checks. At 17:43 AEDT the second restart reported `sys.boot_completed=1`, approximately 5.11 GiB container memory against an 8 GiB limit, `memory.swap.max=0`, zero swap usage, and zero cgroup OOM events. Host `MemAvailable` was 15.63 GiB while a real CI build was also running; these readings are workload-dependent.

Pinned evidence for the source patch:

| Item | Value |
| --- | --- |
| Local image ID | `sha256:809298aa20aa49be169be8a8a94bf9b038208a877724d6481e7739697e802744` |
| Repository digest | `budtmo/docker-android@sha256:1bffbbed5d530284dd355eb73e863411b4a761bc55a00fa1e49fe4a013b6b24a` |
| Original `emulator.py` SHA256 | `ee950b9f6a449ffdb6f4e3ac817cc2d28c61d694d4240cecd07ab1ed8639975e` |
| Patched `emulator.py` SHA256 | `209954c84236a509428e8e2565967868aae36b6aedd4727da5bd0a55b6f9cc40` |

Remove the patched source bind when a tested base image fixes the non-idempotent passwd mutation. Recheck source compatibility before changing the emulator image digest. Audio/video interaction and sustained device test performance were not validated.

### Recovery outcome

The approved SSD correction resolved the flash-permission failure. The emulator guest is running from its persisted AVD; both repeated restarts retained application data. Backups and the original image remain available. The zero-swap guard survives reboot through Unraid cron; it has a first-minute enforcement window after recreation. No host reboot, sustained emulator performance test, or unrelated-service restart was performed.

## Storage and unrelated findings

The protected ZFS pool is 90% allocated with 77% fragmentation and 56.6 GiB dataset-available capacity. The pool is healthy; its 11 October scrub reported zero errors. Main consumers: vault 460 GiB; appdata 226 GiB plus 16.6 GiB snapshots; Docker system 112 GiB including 51.3 GiB child datasets; ISOs 30.2 GiB. The SATA pool has 1.18 TiB available. Root RAM filesystem uses 6.1 GiB; host `/tmp` uses 420 MiB; logs use 15 MiB. Temporary files are not the dominant memory issue.

Docker reports 5.33 GB reclaimable build cache and 6.54 GB reclaimable image storage. These are candidates for a separately reviewed, bounded cleanup after deployment/rollback image needs are known. No storage was deleted. An unreferenced 3.337 GB Docker volume may contain retained application data and must not be blindly pruned. ZFS snapshots and vault contents likewise need retention decisions before removal or migration.

Seerr has approximately 4,037 zombie `wget` children adopted by `npm start`, explaining most of its 4,079 PIDs. These do not consume normal process RSS, but consume process/kernel resources. Its `develop` image and child reaping require a separate bounded fix; no unrelated restart was performed. The `askdit-corpus` container also has a recorded cgroup OOM kill under its 3 GiB cap; it is unrelated to RunnerFarm.

## Durability recommendations

1. Keep explicit, finite per-workload memory and swap budgets. CI memory ceilings need combined JVM/native/page-cache headroom; use zero CI swap with admission, and serialize admission across fleets. Retain a host reserve; don't count full compressed swap as spare capacity.
2. Preserve the actual emulator AVD via a bind mount, persist 4 GiB guest RAM, and validate fresh boot/application retention after recreation. Track actual QEMU/container memory rather than the ignored AVD field.
3. Keep zram as an emergency cushion and monitor its physical allocation, logical headroom, swap rates, and cgroup OOM counters. Alert on sustained pressure and exhausted capacity; enlarging zram alone does not add physical memory.
4. Keep VM allocations unchanged until fresh guest statistics show safe reclaimable capacity. Measure a real CI workload before allowing simultaneous build/dependency fleets.
5. Preserve backups, Docker rollback images, volumes, and snapshots during the reliability deployment. Review build cache cleanup and large vault/ISO placement separately; moving data is not a substitute for fixing RAM overcommit.

Primary references: [Linux zram accounting](https://docs.kernel.org/admin-guide/blockdev/zram.html), [Docker resource constraints](https://docs.docker.com/engine/containers/resource_constraints/), and [Docker stats accounting](https://docs.docker.com/reference/cli/docker/container/stats/).
