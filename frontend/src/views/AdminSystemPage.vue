<template>
  <div class="max-w-6xl mx-auto">
    <div class="flex items-center justify-between mb-6 gap-3">
      <h1 class="text-lg font-semibold">System</h1>
      <div class="flex gap-2">
        <button class="btn btn-ghost btn-sm" @click="loadAll">Refresh</button>
        <button class="btn btn-accent btn-sm" @click="createDefaultSchedule">New backup schedule</button>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4 mb-4">
      <div class="card">
        <div class="card-header">Backup schedules</div>
        <div class="card-body space-y-3">
          <div v-for="schedule in schedules" :key="schedule.id" class="border-b border-[#262626] pb-3 last:border-0 last:pb-0">
            <div class="flex items-center justify-between gap-2">
              <div>
                <div class="text-sm font-medium">{{ schedule.name }}</div>
                <div class="text-xs text-[#737373]">{{ schedule.frequency }} at {{ schedule.time_of_day }}</div>
              </div>
              <span class="badge" :class="schedule.enabled ? 'badge-green' : 'badge-gray'">{{ schedule.enabled ? "Enabled" : "Off" }}</span>
            </div>
            <div class="flex gap-2 mt-2">
              <button class="btn btn-ghost btn-sm" @click="runSchedule(schedule)">Run</button>
              <button class="btn btn-ghost btn-sm" @click="toggleSchedule(schedule)">{{ schedule.enabled ? "Disable" : "Enable" }}</button>
            </div>
          </div>
          <div v-if="!schedules.length" class="empty-state !py-6"><h3>No schedules</h3></div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">Mirror targets</div>
        <div class="card-body space-y-3">
          <div v-for="mirror in mirrors" :key="mirror.id" class="border-b border-[#262626] pb-3 last:border-0 last:pb-0">
            <div class="text-sm font-medium">{{ mirror.name }}</div>
            <div class="text-xs text-[#737373] truncate">{{ mirror.target }}</div>
            <div class="flex items-center justify-between mt-2">
              <span class="badge" :class="mirror.last_status === 'success' ? 'badge-green' : mirror.last_status === 'failed' ? 'badge-red' : 'badge-gray'">{{ mirror.last_status || "Never run" }}</span>
              <button class="btn btn-ghost btn-sm" @click="syncMirror(mirror)">Sync</button>
            </div>
          </div>
          <div v-if="!mirrors.length" class="empty-state !py-6"><h3>No mirrors</h3></div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">Import jobs</div>
        <div class="card-body space-y-2">
          <div v-for="job in imports" :key="job.id" class="text-sm flex items-center justify-between gap-2">
            <div class="min-w-0">
              <div class="truncate">{{ job.target_path }}</div>
              <div class="text-xs text-[#737373]">{{ job.provider }}</div>
            </div>
            <span class="badge" :class="job.status === 'success' ? 'badge-green' : job.status === 'failed' ? 'badge-red' : 'badge-gray'">{{ job.status }}</span>
          </div>
          <div v-if="!imports.length" class="empty-state !py-6"><h3>No import jobs</h3></div>
        </div>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-header">
        <span>Backup jobs</span>
        <span class="text-xs text-[#737373]">{{ jobs.length }} job{{ jobs.length === 1 ? "" : "s" }}</span>
      </div>
      <table v-if="jobs.length">
        <thead><tr><th>Kind</th><th>Status</th><th>Archive</th><th>Started</th><th>Finished</th></tr></thead>
        <tbody>
          <tr v-for="job in jobs" :key="job.id">
            <td>{{ job.kind }}</td>
            <td><span class="badge" :class="job.status === 'success' ? 'badge-green' : job.status === 'failed' ? 'badge-red' : 'badge-gray'">{{ job.status }}</span></td>
            <td class="text-xs text-[#737373] max-w-md truncate">{{ job.archive_path || "-" }}</td>
            <td class="text-xs text-[#737373]">{{ fmt(job.started_at) }}</td>
            <td class="text-xs text-[#737373]">{{ fmt(job.finished_at) }}</td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty-state !py-8"><h3>No backup jobs</h3></div>
    </div>

    <div class="card">
      <div class="card-header">
        <span>Audit log</span>
        <input v-model="auditFilter" placeholder="Filter action..." class="max-w-xs !py-1.5 !text-xs" @keyup.enter="loadAudit" />
      </div>
      <table v-if="audit.length">
        <thead><tr><th>Action</th><th>Actor</th><th>Target</th><th>Message</th><th>Time</th></tr></thead>
        <tbody>
          <tr v-for="event in audit" :key="event.id">
            <td class="font-medium">{{ event.action }}</td>
            <td class="text-sm text-[#737373]">{{ event.actor_username || "system" }}</td>
            <td class="text-xs text-[#737373]">{{ event.target_type }} {{ event.target_id }}</td>
            <td class="text-sm">{{ event.message }}</td>
            <td class="text-xs text-[#737373]">{{ fmt(event.created_at) }}</td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty-state !py-8"><h3>No audit events</h3></div>
    </div>

    <p v-if="error" class="text-xs text-[#dc2626] mt-4">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { api } from "../api/client";

const schedules = ref<any[]>([]);
const jobs = ref<any[]>([]);
const audit = ref<any[]>([]);
const mirrors = ref<any[]>([]);
const imports = ref<any[]>([]);
const auditFilter = ref("");
const error = ref("");

function fmt(value: string) {
  return value ? new Date(value).toLocaleString() : "-";
}

async function loadAudit() {
  const query = auditFilter.value.trim() ? `?action=${encodeURIComponent(auditFilter.value.trim())}` : "";
  audit.value = (await api.get(`/admin/audit-events/${query}`)) || [];
}

async function loadAll() {
  try {
    error.value = "";
    const [scheduleData, jobData, mirrorData, importData] = await Promise.all([
      api.get("/admin/backup-schedules/"),
      api.get("/admin/backup-jobs/"),
      api.get("/admin/mirror-targets/"),
      api.get("/repository-import-jobs/"),
    ]);
    schedules.value = scheduleData || [];
    jobs.value = jobData || [];
    mirrors.value = mirrorData || [];
    imports.value = importData || [];
    await loadAudit();
  } catch (e: any) {
    error.value = e.message;
  }
}

async function createDefaultSchedule() {
  await api.post("/admin/backup-schedules/", {
    name: `Nightly backup ${schedules.value.length + 1}`,
    frequency: "daily",
    time_of_day: "02:15:00",
    enabled: true,
    encrypt: true,
    upload: true,
    keep_local: 14,
  });
  await loadAll();
}

async function toggleSchedule(schedule: any) {
  await api.patch(`/admin/backup-schedules/${schedule.id}/`, { enabled: !schedule.enabled });
  await loadAll();
}

async function runSchedule(schedule: any) {
  await api.post(`/admin/backup-schedules/${schedule.id}/run_now/`);
  await loadAll();
}

async function syncMirror(mirror: any) {
  await api.post(`/admin/mirror-targets/${mirror.id}/sync/`);
  await loadAll();
}

onMounted(loadAll);
</script>
