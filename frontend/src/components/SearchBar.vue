<template>
  <form @submit.prevent="search" class="flex gap-1">
    <input
      v-model="query"
      placeholder="Search..."
      class="px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-700"
    />
    <button type="submit" class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded hover:bg-blue-700">
      Go
    </button>
  </form>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";

const router = useRouter();
const query = ref("");

function search() {
  if (query.value.trim()) {
    router.push(`/search?q=${encodeURIComponent(query.value.trim())}`);
  }
}
</script>
