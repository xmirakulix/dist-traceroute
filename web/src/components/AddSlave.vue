<template>
  <div>
    <h3>Add Slave</h3>
    <div class="add">
      <form>
        <input type="text" v-model="slaveName" placeholder="Add slave..." />
        <button @click.prevent="submitSlave">
          <i class="fas fa-save fa-lg"></i> Save...
        </button>
      </form>
    </div>
  </div>
</template>

<script>
import { mapActions } from "vuex";

export default {
  name: "AddSlave",

  data() {
    return {
      slaveName: ""
    };
  },

  methods: {
    ...mapActions(["addSlave"]),
    submitSlave() {
      this.addSlave(this.slaveName);
    }
  }
};
</script>

<style scoped>
form {
  display: flex;
}

input[type="text"] {
  flex: 10;
  padding: 10px;
  border: 1px solid #484;
  outline: 0;
}

button {
  flex: 2;
  background: #484;
  color: #fff;
  border: 1px solid #484;
  cursor: pointer;
}
</style>
