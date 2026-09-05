<template>
  <div class="fixed bottom-5 right-5 flex flex-col gap-2 z-[200] pointer-events-none">
    <TransitionGroup name="toast">
      <div v-for="t in store.toasts" :key="t.id" :class="{'!bg-[#16a34a]':t.type==='success','!bg-[#dc2626]':t.type==='error','!bg-[#171717]':t.type==='info'}" class="pointer-events-auto bg-[#171717] text-white text-xs px-4 py-2.5 rounded-lg shadow-lg flex items-center gap-2 whitespace-nowrap">
        <span v-if="t.type==='success'">&#10003;</span>
        <span v-else-if="t.type==='error'">&#10007;</span>
        <span v-else>&#8505;</span>
        {{ t.message }}
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
import { useNotificationStore } from "../stores/notification"; const store = useNotificationStore();
</script>

<style scoped>
.toast-enter-active { transition: all .3s ease; }
.toast-leave-active { transition: all .2s ease; }
.toast-enter-from { opacity: 0; transform: translateX(20px); }
.toast-leave-to { opacity: 0; transform: translateX(20px); }
</style>
