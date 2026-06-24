<template>
  <div class="flex gap-2">
    <button
      v-for="p in pages"
      :key="p"
      @click="$emit('page', p)"
      :class="p === current ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-slate-700'"
      class="px-3 py-1 rounded text-sm"
    >
      {{ p }}
    </button>
  </div>
</template>

<script setup lang="ts">
defineProps<{ pages: number[]; current: number }>();
defineEmits<{ page: [value: number] }>();
</script>
