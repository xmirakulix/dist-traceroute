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
              <v-dialog v-model="dialog" max-width="500px">
                <template v-slot:activator="{ on }">
                  <v-btn color="secondary" dark class="mb-2" v-on="on">
                    <v-icon class="mr-2" small>fas fa-plus</v-icon>
                    New Slave
                  </v-btn>
                </template>
                <v-card>
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
                          ></v-text-field>
                        </v-col>
                        <v-col cols="12" sm="6">
                          <v-text-field
                            :type="showPwdInEditDialog ? 'text' : 'password'"
                            v-model="editedItem.Secret"
                            label="Secret"
                            counter
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
                </v-card>
              </v-dialog>
            </v-toolbar>
          </template>

          <!-- action buttons -->
          <template v-slot:item.action="{ item }">
            <v-icon
              small
              class="mr-4"
              @click="editItem(item)"
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
    <v-dialog v-model="deleteDialog" persistent width="unset">
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
          <v-btn
            color="accent"
            @click="
              deleteSlave(editedItem.ID);
              deleteDialog = false;
              editedItem = defaultItem;
              editedIndex = -1;
            "
          >
            Delete
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script>
import { mapActions, mapGetters } from "vuex";

export default {
  name: "ConfigSlaves",

  data() {
    return {
      headers: [
        { text: "ID", value: "ID" },
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
      }
    };
  },

  methods: {
    ...mapActions(["fetchSlaves", "createSlave", "updateSlave", "deleteSlave"]),

    editItem(item) {
      this.editedIndex = this.getSlaves.indexOf(item);
      this.editedItem = Object.assign({}, item);
      this.dialog = true;
    },
    openDeleteDialog(item) {
      this.editedIndex = this.getSlaves.indexOf(item);
      this.editedItem = Object.assign({}, item);
      this.deleteDialog = true;
    },
    close() {
      this.dialog = false;
      this.deleteDialog = false;
      setTimeout(() => {
        this.editedItem = Object.assign({}, this.defaultItem);
        this.editedIndex = -1;
      }, 300);
    },
    save() {
      if (this.editedIndex > -1) {
        this.updateSlave(this.editedItem);
      } else {
        this.createSlave(this.editedItem);
      }
      this.editedItem = Object.assign({}, this.defaultItem);
      this.editedIndex = -1;
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
