<template>
  <div>
    <h1>History of traces</h1>
    <ListTraces :limit="10" />
  </div>
</template>

<script>
import { mapGetters, mapActions } from "vuex";
import ListTraces from "@/components/ListTraces.vue";

export default {
  name: "TraceHistory",

  components: { ListTraces },

  computed: mapGetters(["getStatus"]),

  methods: mapActions(["fetchStatus"]),

  created() {
    this.fetchStatus();
  }
};
</script>

<style scoped></style>
