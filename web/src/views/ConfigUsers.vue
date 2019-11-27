<template>
  <div>
    <h2 class="mt-6">Configured Users</h2>
    <v-card class="mt-4">
      <v-container>
        <v-data-table
          :headers="headers"
          :items="getUsers"
          :items-per-page="5"
          :hide-default-footer="showPages"
          :disable-sort="true"
          class="elevation-1"
        >
          <template v-slot:item.Password="{ item }">
            <span>••••••••••</span>
          </template>

          <template v-slot:item.PasswordNeedsChange="{ item }">
            <v-icon small>
              {{ item.PasswordNeedsChange ? "fas fa-check" : "" }}
            </v-icon>
          </template>

          <!-- add/edit dialog -->
          <template v-slot:top>
            <v-toolbar flat color="white">
              <v-spacer></v-spacer>
              <v-btn color="secondary" dark class="mb-2" @click="openAddDialog">
                <v-icon class="mr-2" small>fas fa-plus</v-icon>
                New User
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
                              label="User name"
                              :rules="rulesName"
                              validate-on-blur
                              autofocus
                            ></v-text-field>
                          </v-col>
                          <v-col cols="12" sm="6">
                            <v-text-field
                              :type="showPwdInEditDialog ? 'text' : 'password'"
                              v-model="editedItem.Password"
                              label="Password"
                              counter
                              :rules="rulesPw"
                              validate-on-blur
                              @focus.self="emtpyOnFocus"
                            >
                              <v-icon
                                slot="append"
                                @click.stop="
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
                        <v-row
                          ><v-col>
                            <v-checkbox
                              v-model="editedItem.PasswordNeedsChange"
                              label="Change password on next login"
                              dense
                            >
                            </v-checkbox></v-col
                        ></v-row>
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
        <v-card-title class="headline"> Delete user</v-card-title>
        <v-card-text>
          Do you really want to delete this user?<br />
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
  name: "ConfigUsers",

  data() {
    return {
      headers: [
        // { text: "ID", value: "ID" },
        { text: "Name", value: "Name" },
        { text: "Password", value: "Password" },
        {
          text: "Change on next Login",
          value: "PasswordNeedsChange"
        },
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
        Password: "",
        PasswordNeedsChange: true
      },
      defaultItem: {
        ID: "",
        Name: "",
        Password: "",
        PasswordNeedsChange: true
      },

      rulesName: [v => v.match(/[^A-Z0-9]/i) == null || "Invalid character"],
      rulesPw: [v => v.length >= 3 || "Minimum length: 6 characters"],

      snack: false,
      snackText: "",
      snackColor: ""
    };
  },

  methods: {
    ...mapActions(["fetchUsers", "createUser", "updateUser", "deleteUser"]),

    openAddDialog() {
      if (this.$refs.addForm != null) {
        this.$refs.addForm.resetValidation();
      }
      this.dialog = true;
    },
    openEditDialog(item) {
      this.editedIndex = this.getUsers.indexOf(item);
      this.editedItem = Object.assign({}, item);
      if (this.$refs.addForm != null) {
        this.$refs.addForm.resetValidation();
      }
      this.dialog = true;
    },

    openDeleteDialog(item) {
      this.editedIndex = this.getUsers.indexOf(item);
      this.editedItem = Object.assign({}, item);
      this.deleteDialog = true;
    },
    doDelete() {
      this.deleteUser(this.editedItem.ID).then(res =>
        res === false
          ? this.doSnack("error", "Error while deleting user!")
          : this.doSnack("success", "Successfully deleted user" + res.Name)
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
        this.updateUser(this.editedItem).then(res =>
          res === false
            ? this.doSnack("error", "Error while updating user!")
            : this.doSnack("success", "Successfully updated user: " + res.Name)
        );
      } else {
        this.createUser(this.editedItem).then(res =>
          res === false
            ? this.doSnack("error", "Error while creating user!")
            : this.doSnack("success", "Successfully created user: " + res.Name)
        );
      }

      this.close();
    },
    emtpyOnFocus() {
      this.editedItem.Password = "";
      this.editedItem.PasswordChanged = true;
    }
  },
  computed: {
    ...mapGetters(["getUsers"]),

    dialogTitle() {
      return this.editedIndex === -1 ? "New User" : "Edit User";
    },
    showPages() {
      return this.getUsers.length < 6 ? true : false;
    }
  },
  created() {
    this.fetchUsers(this.limit);
  }
};
</script>

<style scoped></style>
