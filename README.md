# 📻 Go YouTube Radio Server

> A lightweight, streaming-first HTTP server that converts YouTube/YouTube Music URLs to **MP3 audio streams** — built for **MTA: San Andreas radio mods** and private/local use.

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
![Status](https://img.shields.io/badge/status-in%20development-orange)

⚠️ **For personal/private use only** — does **not** comply with YouTube ToS for public hosting.

---

## 🎯 Features

- ✅ Stream YouTube → MP3 in real-time (no file saved)
- ✅ compatible (`audio/mpeg`, 44.1kHz, stereo)
- ✅ Minimal: single binary, <10 MB RAM
- ✅ Supports `?url=https://youtu.be/...` queries
- ✅ Auto-cleanup on client disconnect
- ✅ `.env` config (`HOST`, `PORT`)
- ✅ Docker & Pterodactyl ready

---