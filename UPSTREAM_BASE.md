# Upstream Base

EasyBox is a fork of 3X-UI. This file records the upstream commit the fork is based on.

| Item | Value |
|---|---|
| Upstream | https://github.com/MHSanaei/3x-ui |
| Branch | main |
| Base commit | a8d0700bf2ed06d77c077fc45fc0387d9abdbaab |
| Commit date | 2026-10-17 |
| Latest release at base | v3.10.2 |
| Recorded | 2026-10-20 |
| Baseline tag | baseline-upstream |

## Remotes

```
upstream  https://github.com/MHSanaei/3x-ui.git
origin    <EasyBox fork, to be added>
```

## Sync procedure

Full procedure, conflict hotspots and post-merge checks: [easybox-docs/UPSTREAM_SYNC.md](easybox-docs/UPSTREAM_SYNC.md).

```
git fetch upstream --tags
git checkout develop && git checkout -b sync/<upstream release tag>
git merge <upstream release tag>
make gen && make verify            # inside devenv, see devenv/README.md
bash devenv/smoke-baseline.sh && bash devenv/smoke-easybox.sh
# merge back to develop, update the table above, release v<upstream>-eb.1
```
