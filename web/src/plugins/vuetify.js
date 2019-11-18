import Vue from "vue";
import Vuetify from "vuetify/lib";

Vue.use(Vuetify);

export default new Vuetify({
  icons: {
    iconfont: "fa"
  },

  theme: {
    themes: {
      light: {
        secondary: "#1976D2",
        accent: "#e91e63",
        primary: "#145CA4",
        success: "#4CAF50",
        info: "#2196F3",
        warning: "#FB8C00",
        error: "#FF5252"
      }
    }
  }
});
