<template>
  <div>
    <h2 class="mt-6">Configured Targets</h2>
    <v-card class="mt-4">
      <v-container>
        <v-data-table
          :headers="headers"
          :items="getTargets"
          :items-per-page="5"
          :hide-default-footer="showPages"
          :disable-sort="true"
          class="elevation-1"
        >
          <!-- add/edit dialog -->
          <template v-slot:top>
            <v-toolbar flat color="white">
              <v-spacer></v-spacer>
              <v-btn color="secondary" dark class="mb-2" @click="openAddDialog">
                <v-icon class="mr-2" small>fas fa-plus</v-icon>
                New Target
              </v-btn>
              <v-dialog
                v-model="dialog"
                max-width="500px"
                @keydown.enter="save"
                @keydown.esc="close"
                @click:outside="close"
              >
                <v-card>
                  <v-form ref="addForm">
                    <v-card-title>
                      <span class="headline">{{ dialogTitle }}</span>
                    </v-card-title>
                    <v-card-text>
                      <v-container>
                        <v-row>
                          <v-col cols="12" sm="6">
                            <v-text-field
                              v-model="editedItem.Name"
                              label="Target name"
                              :rules="rulesName"
                              validate-on-blur
                              autofocus
                            ></v-text-field>
                          </v-col>
                          <v-col cols="12" sm="6">
                            <v-text-field
                              v-model="editedItem.Address"
                              label="Destination address"
                              :rules="rulesAddress"
                              validate-on-blur
                            ></v-text-field>
                          </v-col>
                        </v-row>
                        <v-row>
                          <v-col cols="12" sm="4">
                            <v-text-field
                              v-model.number="editedItem.Retries"
                              label="Retries"
                              :rules="rulesNumber"
                              type="number"
                              validate-on-blur
                            ></v-text-field>
                          </v-col>
                          <v-col cols="12" sm="4">
                            <v-text-field
                              v-model.number="editedItem.MaxHops"
                              label="Maximum hops"
                              :rules="rulesNumber"
                              type="number"
                              validate-on-blur
                            ></v-text-field>
                          </v-col>
                          <v-col cols="12" sm="4">
                            <v-text-field
                              v-model.number="editedItem.TimeoutMs"
                              label="Timeout [mSec]"
                              :rules="rulesNumber"
                              type="number"
                              validate-on-blur
                            ></v-text-field>
                          </v-col>
                        </v-row>
                      </v-container>
                    </v-card-text>
                    <v-card-actions>
                      <v-spacer></v-spacer>
                      <v-btn color="secondary lighten-2" text @click="close"
                        >Cancel</v-btn
                      >
                      <v-btn color="secondary" @click="save">Save</v-btn>
                    </v-card-actions>
                  </v-form>
                </v-card>
              </v-dialog>
            </v-toolbar>
          </template>

          <!-- action buttons -->
          <template v-slot:item.action="{ item }">
            <v-icon
              small
              class="mr-4"
              @click="openEditDialog(item)"
              color="secondary "
            >
              fas fa-pen
            </v-icon>
            <v-icon small @click="openDeleteDialog(item)" color="accent">
              fas fa-trash
            </v-icon>
          </template>
        </v-data-table>
      </v-container>
    </v-card>

    <!-- confirm dialog -->
    <v-dialog
      v-model="deleteDialog"
      persistent
      width="unset"
      @keydown.enter="doDelete"
    >
      <v-card>
        <v-card-title class="headline"> Delete target</v-card-title>
        <v-card-text>
          Do you really want to delete this target?<br />
          Name: <span class="accent--text">{{ editedItem.Name }}</span>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="secondary" text @click="close()">
            Cancel
          </v-btn>
          <v-btn color="accent" @click="doDelete">
            Delete
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Success/error snackbar -->
    <v-snackbar v-model="snack" :timeout="5000" :color="snackColor">
      <v-icon class="mr-2">
        {{
          snackColor == "success"
            ? "fas fa-check-circle"
            : "fas fa-exclamation-circle"
        }}
      </v-icon>
      {{ snackText }}
      <v-btn text @click="snackbar = false">
        Close
      </v-btn>
    </v-snackbar>
  </div>
</template>

<script>
import { mapActions, mapGetters } from "vuex";

export default {
  name: "ConfigTargets",

  data() {
    return {
      headers: [
        // { text: "ID", value: "ID", width: "150px" },
        { text: "Name", value: "Name" },
        { text: "Destination Address", value: "Address" },
        { text: "Retries", value: "Retries", align: "end" },
        { text: "Maximum Hops", value: "MaxHops", align: "end" },
        { text: "Timeout [mSec]", value: "TimeoutMs", align: "end" },
        { text: "", value: "action", sortable: false }
      ],
      dialog: null,
      deleteDialog: false,
      showPwDialog: false,

      showPwdInEditDialog: false,

      editedIndex: -1,
      editedItem: {
        ID: "",
        Name: "",
        Address: "",
        Retries: 1,
        MaxHops: 30,
        TimeoutMs: 500
      },
      defaultItem: {
        ID: "",
        Name: "",
        Address: "",
        Retries: 1,
        MaxHops: 30,
        TimeoutMs: 500
      },

      rulesName: [v => v.match(/[^A-Z0-9]/i) == null || "Invalid character"],
      rulesAddress: [v => v.length >= 6 || "Minimum length: 6 characters"],
      rulesNumber: [v => !!v || "Invalid number"],

      snack: false,
      snackText: "",
      snackColor: ""
    };
  },

  methods: {
    ...mapActions([
      "fetchTargets",
      "createTarget",
      "updateTarget",
      "deleteTarget"
    ]),

    openAddDialog() {
      if (this.$refs.addForm != null) {
        this.$refs.addForm.resetValidation();
      }
      this.dialog = true;
    },
    openEditDialog(item) {
      this.editedIndex = this.getTargets.indexOf(item);
      this.editedItem = Object.assign({}, item);
      if (this.$refs.addForm != null) {
        this.$refs.addForm.resetValidation();
      }
      this.dialog = true;
    },

    openDeleteDialog(item) {
      this.editedIndex = this.getTargets.indexOf(item);
      this.editedItem = Object.assign({}, item);
      this.deleteDialog = true;
    },
    doDelete() {
      this.deleteTarget(this.editedItem.ID).then(res =>
        res === false
          ? this.doSnack("error", "Error while deleting target!")
          : this.doSnack("success", "Successfully deleted target" + res.Name)
      );
      this.deleteDialog = false;
      this.editedItem = this.defaultItem;
      this.editedIndex = -1;
    },
    close() {
      this.dialog = false;
      this.deleteDialog = false;
      setTimeout(() => {
        this.editedItem = Object.assign({}, this.defaultItem);
        this.editedIndex = -1;
      }, 300);
    },
    doSnack(color, text) {
      this.snackColor = color;
      this.snackText = text;
      this.snack = true;
    },
    save() {
      if (!this.$refs.addForm.validate()) {
        return;
      }

      if (this.editedIndex > -1) {
        this.updateTarget(this.editedItem).then(res =>
          res === false
            ? this.doSnack("error", "Error while updating target!")
            : this.doSnack(
                "success",
                "Successfully updated target: " + res.Name
              )
        );
      } else {
        this.createTarget(this.editedItem).then(res =>
          res === false
            ? this.doSnack("error", "Error while creating target!")
            : this.doSnack(
                "success",
                "Successfully created target: " + res.Name
              )
        );
      }

      this.close();
    }
  },
  computed: {
    ...mapGetters(["getTargets"]),

    dialogTitle() {
      return this.editedIndex === -1 ? "New Target" : "Edit Target";
    },
    showPages() {
      return this.getTargets.length < 6 ? true : false;
    }
  },
  created() {
    this.fetchTargets(this.limit);
  }
};
</script>

<style scoped></style>
