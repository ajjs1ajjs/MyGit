<template>
  <div class="flex items-center gap-2 text-sm" v-if="items.length">
    <img v-if="items[0] && 'avatar' in items[0] && items[0].avatar" :src="items[0].avatar" class="w-8 h-8 rounded-full" />
    <div v-else class="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-xs font-bold">
      {{ initials }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{ items: Record<string, any>[] }>();
const initials = computed(() => {
  const name = props.items[0]?.full_name || props.items[0]?.username || "";
  return name.slice(0, 2).toUpperCase();
});
</script>
