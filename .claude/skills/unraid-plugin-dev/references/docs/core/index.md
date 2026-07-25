---
layout: default
title: Core Concepts
nav_order: 10
---

# Core Concepts

This section covers the fundamental building blocks for Unraid plugin development.

## Framework & Configuration

- [Dynamix Framework](docs/core/dynamix-framework.md) - The `$Dynamix` global array and core framework
- [Plugin Settings Storage](docs/core/plugin-settings-storage.md) - `.cfg` files and `parse_plugin_cfg()`
- [Multi-language Support](docs/core/multi-language-support.md) - The `_()` translation function

## Security

- [CSRF Tokens](docs/core/csrf-tokens.md) - Form security requirements

## Real-time & Communication

- [nchan/WebSocket Integration](docs/core/nchan-websocket.md) - Real-time updates
- [Notifications System](docs/core/notifications-system.md) - User alerts and notifications

## Service Management

- [Cron Jobs](docs/core/cron-jobs.md) - Scheduled tasks
- [rc.d Scripts](docs/core/rc-d-scripts.md) - Service management
