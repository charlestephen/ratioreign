<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import { api } from '../api';
import type { QbStatus } from '../types';

const status = ref<QbStatus | null>(null);
let timer: number | undefined;

async function refresh() {
  try {
    status.value = await api<QbStatus>('/api/qbittorrent/status');
  } catch {
    /* keep last good render */
  }
}

function lastPoll(s: QbStatus): string {
  if (!s.lastPollAt || s.lastPollAt === '0001-01-01T00:00:00Z') return 'never yet';
  return new Date(s.lastPollAt).toLocaleString();
}

onMounted(() => {
  refresh();
  timer = window.setInterval(refresh, 5000);
});
onUnmounted(() => window.clearInterval(timer));
</script>

<template>
  <div v-if="status?.enabled" class="card">
    <h2>qBittorrent sync</h2>
    <table>
      <tr><th class="k">Last poll</th><td>{{ lastPoll(status) }}</td></tr>
      <tr><th class="k">Torrents tracked</th><td>{{ status.torrentsTracked ?? 0 }}</td></tr>
      <tr><th class="k">Last error</th><td>{{ status.lastError || 'none' }}</td></tr>
    </table>

    <template v-if="status.excluded?.length">
      <h3>Excluded by sync rules</h3>
      <table>
        <thead>
          <tr><th>Name</th><th>Reason</th></tr>
        </thead>
        <tbody>
          <tr v-for="e in status.excluded" :key="e.hash">
            <td>{{ e.name }}</td>
            <td>{{ e.reason }}</td>
          </tr>
        </tbody>
      </table>
    </template>
  </div>
</template>

<style scoped>
.k { width: 160px; }
h3 { font-size: 13px; margin: 16px 0 8px; }
</style>
