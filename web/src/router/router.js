import Vue from "vue";
import VueRouter from "vue-router";
import Home from "@/views/Home.vue";

Vue.use(VueRouter);

const routes = [
  {
    path: "/",
    name: "home",
    component: Home
  },
  {
    path: "/config/users",
    name: "configUsers",
    component: () =>
      import(/* webpackChunkName: "configUsers" */ "@/views/ConfigUsers.vue")
  },
  {
    path: "/config/slaves",
    name: "configSlaves",
    component: () =>
      import(/* webpackChunkName: "configSlaves" */ "@/views/ConfigSlaves.vue")
  },
  {
    path: "/config/targets",
    name: "configTargets",
    component: () =>
      import(
        /* webpackChunkName: "configTargets" */ "@/views/ConfigTargets.vue"
      )
  },
  {
    path: "/alerts",
    name: "alerts",
    component: () =>
      import(/* webpackChunkName: "alerts" */ "@/views/AlertHistory.vue")
  },
  {
    path: "/history",
    name: "history",
    component: () =>
      import(/* webpackChunkName: "history" */ "@/views/TraceHistory.vue")
  },
  {
    path: "/graph/:destID/:slaveID",
    name: "graph",
    // props: true,
    component: () =>
      import(/* webpackChunkName: "graph" */ "@/views/TraceGraph.vue")
  }
];

const router = new VueRouter({
  routes
});

export default router;
