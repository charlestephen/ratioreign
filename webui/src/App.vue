<script setup lang="ts">
import TorrentsCard from './components/TorrentsCard.vue';
import QbStatusCard from './components/QbStatusCard.vue';
import ConfigForm from './components/ConfigForm.vue';
</script>

<template>
  <h1>RatioReign</h1>
  <p class="sub">Fake-seeding tracker daemon — status and configuration.</p>
  <TorrentsCard />
  <QbStatusCard />
  <ConfigForm />
</template>
