import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./router";
import "./style.css";

console.log("MyGit: mounting...");
const app = createApp(App);
app.use(createPinia());
app.use(router);
app.mount("#app");
console.log("MyGit: mounted OK");
