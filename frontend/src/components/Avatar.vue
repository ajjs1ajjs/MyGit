<template>
  <div class="flex items-center gap-2 text-sm">
    <img v-if="user?.avatar" :src="user.avatar" class="w-8 h-8 rounded-full" />
    <div v-else class="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-xs font-bold">
      {{ initials }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{ user?: { full_name?: string; username?: string; avatar?: string } }>();
const initials = computed(() => {
  const name = props.user?.full_name || props.user?.username || "";
  return name.slice(0, 2).toUpperCase();
});
</script>
