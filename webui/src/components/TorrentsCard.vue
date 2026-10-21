<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { api, humanBytes } from '../api';
import type { TorrentStatus } from '../types';

type SortKey = 'name' | 'uploaded' | 'ratio' | 'seeders' | 'leechers' | 'active' | 'lastError';

const torrents = ref<TorrentStatus[]>([]);
const sortKey = ref<SortKey>('name');
const sortDir = ref<1 | -1>(1);
const filters = ref({ name: '', uploadedMB: '', ratio: '', seeders: '', leechers: '', active: '', lastError: '' });
const fileInput = ref<HTMLInputElement | null>(null);
const uploadMsg = ref<{ text: string; ok: boolean } | null>(null);
let timer: number | undefined;

const filtered = computed(() =>
  torrents.value.filter((t) => {
    const f = filters.value;
    if (f.name && !t.name.toLowerCase().includes(f.name.toLowerCase())) return false;
    if (f.lastError && !(t.lastError ?? '').toLowerCase().includes(f.lastError.toLowerCase())) return false;
    if (f.uploadedMB !== '' && t.uploaded < Number(f.uploadedMB) * 1024 * 1024) return false;
    if (f.ratio !== '' && t.ratio < Number(f.ratio)) return false;
    if (f.seeders !== '' && t.seeders < Number(f.seeders)) return false;
    if (f.leechers !== '' && t.leechers < Number(f.leechers)) return false;
    if (f.active !== '' && String(t.active) !== f.active) return false;
    return true;
  }),
);

const rows = computed(() => {
  const key = sortKey.value;
  const mul = sortDir.value;
  return [...filtered.value].sort((a, b) => {
    let av: string | number = a[key] ?? '';
    let bv: string | number = b[key] ?? '';
    if (typeof av === 'boolean') av = av ? 1 : 0;
    if (typeof bv === 'boolean') bv = bv ? 1 : 0;
    if (typeof av === 'string') {
      av = av.toLowerCase();
      bv = String(bv).toLowerCase();
    }
    return av < bv ? -mul : av > bv ? mul : 0;
  });
});

function toggleSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 1 ? -1 : 1;
  } else {
    sortKey.value = key;
    sortDir.value = 1;
  }
}

function arrow(key: SortKey): string {
  return sortKey.value === key ? (sortDir.value === 1 ? '▲' : '▼') : '';
}

async function refresh() {
  try {
    torrents.value = await api<TorrentStatus[]>('/api/torrents');
  } catch {
    /* keep last good render; next tick retries */
  }
}

async function remove(hash: string, ev: MouseEvent) {
  const btn = ev.currentTarget as HTMLButtonElement;
  btn.disabled = true;
  try {
    await api(`/api/torrents/${hash}`, { method: 'DELETE' });
    await refresh();
  } finally {
    btn.disabled = false;
  }
}

async function upload() {
  const file = fileInput.value?.files?.[0];
  if (!file) return;
  const form = new FormData();
  form.append('torrent', file);
  uploadMsg.value = { text: 'Uploading…', ok: true };
  try {
    await api('/api/torrents', { method: 'POST', body: form });
    uploadMsg.value = { text: 'Added.', ok: true };
    if (fileInput.value) fileInput.value.value = '';
    await refresh();
  } catch (e) {
    uploadMsg.value = { text: (e as Error).message, ok: false };
  }
}

onMounted(() => {
  refresh();
  timer = window.setInterval(refresh, 3000);
});
onUnmounted(() => window.clearInterval(timer));
</script>

<template>
  <div class="card">
    <h2>Torrents</h2>
    <table>
      <thead>
        <tr>
          <th class="sortable" @click="toggleSort('name')">Name <span class="arrow">{{ arrow('name') }}</span></th>
          <th class="sortable" @click="toggleSort('uploaded')">Uploaded <span class="arrow">{{ arrow('uploaded') }}</span></th>
          <th class="sortable" @click="toggleSort('ratio')">Ratio <span class="arrow">{{ arrow('ratio') }}</span></th>
          <th class="sortable" @click="toggleSort('seeders')">Seeders <span class="arrow">{{ arrow('seeders') }}</span></th>
          <th class="sortable" @click="toggleSort('leechers')">Leechers <span class="arrow">{{ arrow('leechers') }}</span></th>
          <th class="sortable" @click="toggleSort('active')">Active <span class="arrow">{{ arrow('active') }}</span></th>
          <th class="sortable" @click="toggleSort('lastError')">Last error <span class="arrow">{{ arrow('lastError') }}</span></th>
          <th></th>
        </tr>
        <tr class="filter-row">
          <th><input v-model="filters.name" type="text" placeholder="filter…"></th>
          <th><input v-model="filters.uploadedMB" type="number" placeholder="min MB" min="0"></th>
          <th><input v-model="filters.ratio" type="number" placeholder="min" min="0" step="0.01"></th>
          <th><input v-model="filters.seeders" type="number" placeholder="min" min="0"></th>
          <th><input v-model="filters.leechers" type="number" placeholder="min" min="0"></th>
          <th>
            <select v-model="filters.active">
              <option value="">any</option>
              <option value="true">active</option>
              <option value="false">waiting</option>
            </select>
          </th>
          <th><input v-model="filters.lastError" type="text" placeholder="filter…"></th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="rows.length === 0">
          <td colspan="8" class="empty">
            {{ torrents.length ? 'No torrents match the current filters.' : 'No torrents yet — upload one below, or enable RSS/qBittorrent intake.' }}
          </td>
        </tr>
        <tr v-for="t in rows" :key="t.infoHash">
          <td>{{ t.name }}</td>
          <td>{{ humanBytes(t.uploaded) }}</td>
          <td>{{ t.ratio.toFixed(3) }}</td>
          <td>{{ t.seeders }}</td>
          <td>{{ t.leechers }}</td>
          <td>{{ t.active ? 'yes' : 'waiting' }}</td>
          <td>{{ t.lastError ?? '' }}</td>
          <td><button class="danger small" @click="remove(t.infoHash, $event)">Remove</button></td>
        </tr>
      </tbody>
    </table>

    <form class="upload" @submit.prevent="upload">
      <input ref="fileInput" type="file" accept=".torrent" required>
      <button type="submit">Upload .torrent</button>
      <span v-if="uploadMsg" class="msg" :class="uploadMsg.ok ? 'ok' : 'err'">{{ uploadMsg.text }}</span>
    </form>
  </div>
</template>

<style scoped>
.upload { margin-top: 16px; display: flex; gap: 8px; align-items: center; }
</style>
