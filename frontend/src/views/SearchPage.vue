<template>
  <div class="max-w-4xl mx-auto">
      <h2 class="font-semibold mb-3">Search</h2>
      <p v-if="!results" class="text-gray-500">Enter a query to search.</p>
      <div v-for="(items, category) in results" :key="category" class="mb-4">
        <h3 class="text-sm font-semibold text-gray-500 uppercase mb-2">{{ category }}</h3>
        <div class="border rounded divide-y">
          <div v-for="item in items" :key="item.id" class="px-4 py-2 text-sm">{{ item.title || item.path || item.name }}</div>
        </div>
      </div>
    </div>
  </template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
const route = useRoute();
const results = ref<any>(null);

onMounted(async () => {
  const q = route.query.q as string;
  if (q) {
    try {
      results.value = await api.get(`/search?q=${encodeURIComponent(q)}`);
    } catch {}
  }
});
</script>
