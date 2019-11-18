<template>
  <div>
    <h2 class="mt-6">Configured Slaves</h2>
    <v-card>
      <v-container>
        <v-data-table
          :headers="headers"
          :items="getSlaves"
          :items-per-page="5"
          :hide-default-footer="limit <= 3"
          :disable-sort="true"
          class="elevation-1"
        >
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
                            :type="showPwd ? 'text' : 'password'"
                            v-model="editedItem.Password"
                            label="Password"
                            counter
                          >
                            <v-icon
                              slot="append"
                              @click="showPwd = !showPwd"
                              small
                              class="mt-1"
                            >
                              {{
                                showPwd
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
          <template v-slot:item.action="{ item }">
            <v-icon
              small
              class="mr-4"
              @click="editItem(item)"
              color="secondary "
            >
              fas fa-pen
            </v-icon>
            <v-icon small @click="deleteItem(item)" color="error">
              fas fa-trash
            </v-icon>
          </template>
        </v-data-table>
      </v-container>
    </v-card>
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
        { text: "Password", value: "Password" },
        { text: "", value: "action", sortable: false }
      ],
      dialog: null,
      showPwd: false,

      editedIndex: -1,
      editedItem: {
        ID: "",
        Name: "",
        Password: ""
      },
      defaultItem: {
        ID: "",
        Name: "",
        Password: ""
      }
    };
  },

  props: {
    limit: {
      type: Number,
      default: 3
    }
  },

  methods: {
    ...mapActions(["fetchSlaves"]),

    editItem(item) {
      this.editedIndex = this.getSlaves.indexOf(item);
      this.editedItem = Object.assign({}, item);
      this.dialog = true;
    },
    deleteItem(item) {
      const index = this.getSlaves.indexOf(item);
      confirm("Are you sure you want to delete this slave?") &&
        this.getSlaves.splice(index, 1);
    },
    close() {
      this.dialog = false;
      setTimeout(() => {
        this.editedItem = Object.assign({}, this.defaultItem);
        this.editedIndex = -1;
      }, 300);
    },
    save() {
      if (this.editedIndex > -1) {
        Object.assign(this.getSlaves[this.editedIndex], this.editedItem);
      } else {
        this.getSlaves.push(this.editedItem);
      }
      this.close();
    }
  },
  computed: {
    ...mapGetters(["getSlaves"]),

    dialogTitle() {
      return this.editedIndex === -1 ? "New Slave" : "Edit Slave";
    }
  },
  created() {
    this.fetchSlaves(this.limit);
  }
};
</script>

<style scoped></style>
