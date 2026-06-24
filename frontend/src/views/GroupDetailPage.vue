<template>
  <div class="max-w-4xl mx-auto">
      <h2 class="font-semibold mb-3">{{ group?.path }}</h2>
      <div v-if="projects.length" class="grid gap-3">
        <RouterLink v-for="p in projects" :key="p.id" :to="`/${p.path}`" class="p-4 border rounded-lg block">{{ p.name }}</RouterLink>
      </div>
    </div>
  </template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute } from "vue-router";
import { api } from "../api/client";
const route = useRoute();
const group = ref<any>(null);
const projects = ref<any[]>([]);
onMounted(async () => {
  try {
    group.value = await api.get(`/groups/${route.params.id}/`);
    projects.value = await api.get(`/groups/${route.params.id}/projects/`) || [];
  } catch {}
});
</script>
