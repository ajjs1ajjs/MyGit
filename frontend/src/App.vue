<template>
  <div style="padding:40px;font-family:sans-serif">
    <h1 style="color:#2563eb">MyGit OK</h1>
    <p>Vue mounted successfully</p>
    <p v-if="error" style="color:red">{{ error }}</p>
    <p v-else>Router: working</p>
    <router-view />
  </div>
</template>

<script setup lang="ts">
import { onErrorCaptured, ref } from "vue";
const error = ref("");
onErrorCaptured((e) => { error.value = String(e); return false; });
</script>
