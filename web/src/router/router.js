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
    path: "/history",
    name: "history",
    component: () =>
      import(/* webpackChunkName: "history" */ "@/views/TraceHistory.vue")
  },
  {
    path: "/graph/:id",
    name: "graph",
    props: true,
    component: () =>
      import(/* webpackChunkName: "graph" */ "@/views/TraceGraph.vue")
  }
];

const router = new VueRouter({
  routes
});

export default router;
