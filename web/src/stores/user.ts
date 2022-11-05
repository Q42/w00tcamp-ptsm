import { ref, computed } from "vue";
import { defineStore } from "pinia";
import { auth, db } from "./db";
import { signOut } from "firebase/auth";
import {
  collection,
  doc,
  getDoc,
  getDocs,
  onSnapshot,
  Query,
  query,
  where,
  type Unsubscribe,
} from "firebase/firestore";

export const email = ref<string | null>(null);
export const loading = ref(true);
export const mailboxId = ref<string | null>(null);

let cancelListener: Unsubscribe | null = null;

export const logout = async () => {
  loading.value = true;
  await signOut(auth);
  loading.value = false;
};

auth.onAuthStateChanged(async (user) => {
  console.log("Logged in", user?.email);
  if (cancelListener) {
    cancelListener();
  }
  if (user) {
    email.value = user.email;

    cancelListener = onSnapshot(
      query(collection(db, "mailboxes"), where("user", "==", user.email)),
      (mailboxDoc) => {
        if (mailboxDoc.size > 1) {
          throw new Error("multiple mailboxes found for " + user.email);
        } else if (!mailboxDoc.empty) {
          mailboxId.value = mailboxDoc.docs[0].id;
        } else {
          mailboxId.value = null;
        }
        loading.value = false;
      }
    );
  } else {
    email.value = null;
    mailboxId.value = null;
    loading.value = false;
  }
});
