<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, jsonPost } from '../api';
import type { Config, QbExclude } from '../types';

const cfg = ref<Config | null>(null);
const profiles = ref<string[]>([]);
const loadError = ref('');
const saveMsg = ref<{ text: string; ok: boolean } | null>(null);
const testMsg = ref<{ text: string; ok: boolean } | null>(null);
const trackersText = ref('');
const tagsText = ref('');

const emptyExclude = (): QbExclude => ({ trackers: [], tags: [], maxSeeders: 0, minLeechers: 0, evict: false });

function linesToList(text: string): string[] {
  return text.split('\n').map((s) => s.trim()).filter(Boolean);
}

async function load() {
  try {
    const [c, p] = await Promise.all([api<Config>('/api/config'), api<string[]>('/api/profiles')]);
    if (c.qbittorrent && !c.qbittorrent.exclude) c.qbittorrent.exclude = emptyExclude();
    cfg.value = c;
    profiles.value = p;
    trackersText.value = c.qbittorrent?.exclude?.trackers.join('\n') ?? '';
    tagsText.value = c.qbittorrent?.exclude?.tags.join('\n') ?? '';
  } catch (e) {
    loadError.value = 'Failed to load config: ' + (e as Error).message;
  }
}

async function testConnection() {
  if (!cfg.value?.qbittorrent) return;
  testMsg.value = { text: 'Testing…', ok: true };
  try {
    const result = await api<{ success: boolean; message: string }>(
      '/api/qbittorrent/test',
      jsonPost({
        url: cfg.value.qbittorrent.url,
        username: cfg.value.qbittorrent.username,
        password: cfg.value.qbittorrent.password,
      }),
    );
    testMsg.value = { text: result.message, ok: result.success };
  } catch (e) {
    testMsg.value = { text: (e as Error).message, ok: false };
  }
}

async function waitForRestart() {
  await new Promise((r) => setTimeout(r, 1500));
  for (let i = 0; i < 30; i++) {
    try {
      const resp = await fetch('/healthz');
      if (resp.ok) return;
    } catch { /* process is down for restart */ }
    await new Promise((r) => setTimeout(r, 1000));
  }
}

async function save() {
  if (!cfg.value) return;
  if (cfg.value.qbittorrent?.exclude) {
    cfg.value.qbittorrent.exclude.trackers = linesToList(trackersText.value);
    cfg.value.qbittorrent.exclude.tags = linesToList(tagsText.value);
  }
  saveMsg.value = { text: 'Saving…', ok: true };
  try {
    await api('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg.value),
    });
  } catch (e) {
    saveMsg.value = { text: (e as Error).message, ok: false };
    return;
  }
  saveMsg.value = { text: 'Saved — restarting…', ok: true };
  await waitForRestart();
  location.reload();
}

onMounted(load);
</script>

<template>
  <div class="card">
    <h2>Configuration</h2>
    <p v-if="loadError" class="msg err">{{ loadError }}</p>
    <form v-if="cfg" @submit.prevent="save">
      <div class="row">
        <div>
          <label>API listen address</label>
          <input v-model="cfg.listenAddr" type="text" required>
        </div>
        <div>
          <label>Client profile</label>
          <select v-model="cfg.clientProfile" required>
            <option v-for="p in profiles" :key="p" :value="p">{{ p }}</option>
          </select>
        </div>
      </div>

      <div class="row">
        <div>
          <label>Min upload rate (kB/s)</label>
          <input v-model.number="cfg.minUploadRateKBs" type="number" min="0" required>
        </div>
        <div>
          <label>Max upload rate (kB/s)</label>
          <input v-model.number="cfg.maxUploadRateKBs" type="number" min="0" required>
        </div>
        <div>
          <label>Simultaneous seeds</label>
          <input v-model.number="cfg.simultaneousSeed" type="number" min="1" required>
        </div>
        <div>
          <label>Upload ratio target (-1 = disabled)</label>
          <input v-model.number="cfg.uploadRatioTarget" type="number" step="0.01" required>
        </div>
      </div>

      <label class="inline">
        <input v-model="cfg.keepTorrentWithZeroLeechers" type="checkbox">
        Keep seeding torrents with zero leechers
      </label>

      <fieldset>
        <legend>Paths</legend>
        <p class="hint">Container paths — changing these without updating your volume mounts will break things.</p>
        <div class="row">
          <div><label>Torrents dir (watched folder)</label><input v-model="cfg.torrentsDir" type="text" required></div>
          <div><label>Archive dir</label><input v-model="cfg.archiveDir" type="text" required></div>
          <div><label>Profiles dir</label><input v-model="cfg.profilesDir" type="text" required></div>
          <div><label>Ratio state file</label><input v-model="cfg.statePath" type="text" required></div>
        </div>
      </fieldset>

      <fieldset>
        <legend>RSS feeds</legend>
        <div v-for="(feed, i) in cfg.rss" :key="i" class="feed-row">
          <div class="name"><input v-model="feed.name" type="text" placeholder="name"></div>
          <div><input v-model="feed.url" type="url" placeholder="https://tracker.example/rss?key=..."></div>
          <div class="interval"><input v-model="feed.pollInterval" type="text" placeholder="10m"></div>
          <div style="flex: 0"><button type="button" class="danger small" @click="cfg.rss.splice(i, 1)">✕</button></div>
        </div>
        <button type="button" class="secondary small" @click="cfg.rss.push({ name: '', url: '', pollInterval: '' })">+ Add feed</button>
      </fieldset>

      <fieldset v-if="cfg.qbittorrent">
        <legend>qBittorrent sync</legend>
        <label class="inline"><input v-model="cfg.qbittorrent.enabled" type="checkbox"> Enabled</label>
        <div class="row">
          <div><label>URL</label><input v-model="cfg.qbittorrent.url" type="url" placeholder="http://localhost:8080"></div>
          <div><label>Username</label><input v-model="cfg.qbittorrent.username" type="text"></div>
          <div>
            <label>Password</label>
            <input v-model="cfg.qbittorrent.password" type="password" :placeholder="cfg.qbittorrent.passwordSet ? '(unchanged — leave blank to keep)' : ''">
          </div>
        </div>
        <p v-if="cfg.qbittorrent.passwordSet" class="hint">A password is currently saved; leave blank to keep it.</p>
        <div class="row">
          <div><label>Poll interval</label><input v-model="cfg.qbittorrent.pollInterval" type="text" placeholder="30s"></div>
          <div><label>Category filter (optional)</label><input v-model="cfg.qbittorrent.category" type="text"></div>
        </div>
        <div class="footer-actions">
          <button type="button" class="secondary" @click="testConnection">Test connection</button>
          <span v-if="testMsg" class="msg" :class="testMsg.ok ? 'ok' : 'err'">{{ testMsg.text }}</span>
        </div>

        <fieldset v-if="cfg.qbittorrent.exclude">
          <legend>Exclusions</legend>
          <p class="hint">
            Matching torrents are not imported. Seeder/leecher thresholds use the tracker's swarm
            counts. Excluded torrents are re-checked every poll and imported if they stop matching.
          </p>
          <div class="row">
            <div>
              <label>Excluded tracker hosts (one per line, substring)</label>
              <textarea v-model="trackersText" rows="3" placeholder="tracker.example.org"></textarea>
            </div>
            <div>
              <label>Excluded tags (one per line, exact)</label>
              <textarea v-model="tagsText" rows="3" placeholder="donotseed"></textarea>
            </div>
          </div>
          <div class="row">
            <div>
              <label>Max swarm seeders (0 = off)</label>
              <input v-model.number="cfg.qbittorrent.exclude.maxSeeders" type="number" min="0">
            </div>
            <div>
              <label>Min swarm leechers (0 = off)</label>
              <input v-model.number="cfg.qbittorrent.exclude.minLeechers" type="number" min="0">
            </div>
          </div>
          <label class="inline">
            <input v-model="cfg.qbittorrent.exclude.evict" type="checkbox">
            Also evict already-imported torrents that start matching (stops seeding and removes the file)
          </label>
        </fieldset>
      </fieldset>

      <div class="footer-actions">
        <button type="submit">Save &amp; restart</button>
        <span v-if="saveMsg" class="msg" :class="saveMsg.ok ? 'ok' : 'err'">{{ saveMsg.text }}</span>
      </div>
    </form>
  </div>
</template>
