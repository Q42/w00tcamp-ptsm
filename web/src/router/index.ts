import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import SettingsView from "../views/SettingsView.vue";
import PayMailView from "../views/PayMailView.vue";
import PaidMailView from "../views/PaidMailView.vue";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/",
      name: "home",
      component: HomeView,
    },
    {
      path: "/settings",
      name: "settings",
      component: SettingsView,
    },
    {
      path: "/pay/:recipient/:emailId",
      name: "pay",
      component: PayMailView,
      props: true,
    },
    {
      path: "/paid/:recipient/:emailId",
      name: "paid",
      component: PaidMailView,
      props: true,
    },
  ],
});

export default router;
