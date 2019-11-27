<template>
  <div>
    <h2 class="mt-6">Configured Slaves</h2>
    <v-card class="mt-4">
      <v-container>
        <v-data-table
          :headers="headers"
          :items="getSlaves"
          :items-per-page="5"
          :hide-default-footer="showPages"
          :disable-sort="true"
          class="elevation-1"
        >
          <template v-slot:item.Secret="{ item }">
            <v-dialog width="auto ">
              <template v-slot:activator="{ on }">
                <span>••••••••</span>
                <v-icon class="ml-2" slot="append" small v-on="on">
                  fas fa-eye
                </v-icon>
              </template>
              <v-card>
                <v-container>
                  <v-row>
                    <v-col class="mx-4">Secret: {{ item.Secret }} </v-col>
                  </v-row>
                </v-container>
              </v-card>
            </v-dialog>
          </template>

          <!-- add/edit dialog -->
          <template v-slot:top>
            <v-toolbar flat color="white">
              <v-spacer></v-spacer>
              <v-btn color="secondary" dark class="mb-2" @click="openAddDialog">
                <v-icon class="mr-2" small>fas fa-plus</v-icon>
                New Slave
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
                              label="Slave name"
                              :rules="rulesName"
                              validate-on-blur
                              autofocus
                            ></v-text-field>
                          </v-col>
                          <v-col cols="12" sm="6">
                            <v-text-field
                              :type="showPwdInEditDialog ? 'text' : 'password'"
                              v-model="editedItem.Secret"
                              label="Secret"
                              counter
                              :rules="rulesSecret"
                              validate-on-blur
                            >
                              <v-icon
                                slot="append"
                                @click="
                                  showPwdInEditDialog = !showPwdInEditDialog
                                "
                                small
                                class="mt-1"
                              >
                                {{
                                  showPwdInEditDialog
                                    ? "fas fa-eye-slash fa-xs"
                                    : "fas fa-eye fa-xs"
                                }}
                              </v-icon>
                            </v-text-field>
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
        <v-card-title class="headline"> Delete slave</v-card-title>
        <v-card-text>
          Do you really want to delete this slave?<br />
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
  name: "ConfigSlaves",

  data() {
    return {
      headers: [
        // { text: "ID", value: "ID" },
        { text: "Name", value: "Name" },
        { text: "Secret", value: "Secret" },
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
        Secret: ""
      },
      defaultItem: {
        ID: "",
        Name: "",
        Secret: ""
      },

      rulesName: [v => v.match(/[^A-Z0-9]/i) == null || "Invalid character"],
      rulesSecret: [v => v.length >= 6 || "Minimum length: 6 characters"],

      snack: false,
      snackText: "",
      snackColor: ""
    };
  },

  methods: {
    ...mapActions(["fetchSlaves", "createSlave", "updateSlave", "deleteSlave"]),

    openAddDialog() {
      if (this.$refs.addForm != null) {
        this.$refs.addForm.resetValidation();
      }
      this.dialog = true;
    },
    openEditDialog(item) {
      this.editedIndex = this.getSlaves.indexOf(item);
      this.editedItem = Object.assign({}, item);
      if (this.$refs.addForm != null) {
        this.$refs.addForm.resetValidation();
      }
      this.dialog = true;
    },

    openDeleteDialog(item) {
      this.editedIndex = this.getSlaves.indexOf(item);
      this.editedItem = Object.assign({}, item);
      this.deleteDialog = true;
    },
    doDelete() {
      this.deleteSlave(this.editedItem.ID).then(res =>
        res === false
          ? this.doSnack("error", "Error while deleting slave!")
          : this.doSnack("success", "Successfully deleted slave" + res.Name)
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
        this.updateSlave(this.editedItem).then(res =>
          res === false
            ? this.doSnack("error", "Error while updating slave!")
            : this.doSnack("success", "Successfully updated slave: " + res.Name)
        );
      } else {
        this.createSlave(this.editedItem).then(res =>
          res === false
            ? this.doSnack("error", "Error while creating slave!")
            : this.doSnack("success", "Successfully created slave: " + res.Name)
        );
      }

      this.close();
    }
  },
  computed: {
    ...mapGetters(["getSlaves"]),

    dialogTitle() {
      return this.editedIndex === -1 ? "New Slave" : "Edit Slave";
    },
    showPages() {
      return this.getSlaves.length < 6 ? true : false;
    }
  },
  created() {
    this.fetchSlaves(this.limit);
  }
};
</script>

<style scoped></style>
