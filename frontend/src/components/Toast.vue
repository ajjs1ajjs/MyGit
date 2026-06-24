<template>
  <div class="fixed bottom-4 right-4 flex flex-col gap-2 z-50">
    <div
      v-for="t in store.toasts"
      :key="t.id"
      :class="{
        'bg-green-600': t.type === 'success',
        'bg-red-600': t.type === 'error',
        'bg-blue-600': t.type === 'info',
      }"
      class="text-white px-4 py-2 rounded shadow-lg text-sm"
    >
      {{ t.message }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { useNotificationStore } from "../stores/notification";
const store = useNotificationStore();
</script>
