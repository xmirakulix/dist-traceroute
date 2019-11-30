<template>
  <div>
    <h2 class="mt-6">Last alerts</h2>
    <v-container>
      <v-data-table
        :headers="headers"
        :items="lastAlerts"
        :items-per-page="5"
        :hide-default-footer="limit <= 3"
        :disable-sort="true"
        class="elevation-1"
      >
        <template v-slot:item.Severity="{ item }">
          <v-icon small :color="item.Severity == 'info' ? '' : item.Severity">
            {{ getSeverityIcon(item.Severity) }}
          </v-icon>
        </template>

        <template v-slot:item.Time="{ item }">
          {{ getDisplayTime(item.Time) }}
        </template>
      </v-data-table>
    </v-container>
  </div>
</template>

<script>
import { mapGetters } from "vuex";

export default {
  name: "ListAlerts",

  data() {
    return {
      headers: [
        { text: "", value: "Severity", width: "50px" },
        { text: "Time", value: "Time" },
        { text: "Source", value: "Source" },
        { text: "Text", value: "Text" }
      ]
    };
  },

  props: {
    limit: {
      type: Number,
      default: 3
    }
  },

  computed: {
    ...mapGetters(["getStatus"]),

    lastAlerts() {
      if (this.getStatus.LastAlerts) {
        return this.getStatus.LastAlerts.slice(-1 * this.limit);
      }
      return Array();
    }
  },

  methods: {
    getDisplayTime(time) {
      var date = new Date(time);

      return date.toLocaleDateString() + " " + date.toLocaleTimeString();
    },

    getSeverityIcon(severity) {
      switch (severity) {
        case "info":
          return "fas fa-info-circle";
        case "warning":
          return "fas fa-exclamation-triangle";
        case "error":
          return "fas fa-exclamation-triangle";
      }
    }
  }
};
</script>

<style scoped></style>
