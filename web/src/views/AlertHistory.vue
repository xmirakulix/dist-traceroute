<template>
  <div>
    <h1>History of alerts</h1>
    <ListAlerts :limit="10" />
  </div>
</template>

<script>
//import { mapGetters, mapActions } from "vuex";
import ListAlerts from "@/components/ListAlerts.vue";

export default {
  name: "AlertHistory",

  components: { ListAlerts }
};
</script>

<style scoped></style>
