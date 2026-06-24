<template>
  <AppLayout>
    <div class="max-w-4xl mx-auto">
      <h2 class="font-semibold mb-3">Groups</h2>
      <div v-if="groups.length" class="grid gap-3">
        <RouterLink v-for="g in groups" :key="g.id" :to="`/groups/${g.id}`" class="p-4 border rounded-lg block hover:shadow">
          <h3 class="font-semibold">{{ g.path }}</h3>
          <p class="text-sm text-gray-500">{{ g.description || g.name }}</p>
        </RouterLink>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import AppLayout from "../components/AppLayout.vue";

const groups = ref<any[]>([]);
onMounted(async () => {
  try {
    groups.value = await api.get("/groups/") || [];
  } catch {}
});
</script>
