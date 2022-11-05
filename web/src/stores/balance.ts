import { ref, computed, watch } from "vue";
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
  Timestamp,
  where,
  type Unsubscribe,
} from "firebase/firestore";
import { email } from "./user";

interface Balance {
  balance: number;
  paymentLink: string;
  added: Timestamp;
}
interface Transaction {
  plus?: number;
  minus?: number;
  added: Timestamp;
}

export const balance = ref<Balance | null>(null);
export const transactions = ref<Transaction[]>([]);

let cancelBalanceListener: Unsubscribe | null = null;
let cancelTransactionListener: Unsubscribe | null = null;

watch(email, (newEmail) => {
  if (cancelBalanceListener) {
    cancelBalanceListener();
    balance.value = null;
  }
  if (cancelTransactionListener) {
    cancelTransactionListener();
    transactions.value = [];
  }

  if (newEmail) {
    const balanceRef = doc(db, "balance", newEmail);
    cancelBalanceListener = onSnapshot(balanceRef, (update) => {
      if (update.exists()) {
        balance.value = update.data() as Balance;
        console.log("[db] balance", balance.value);
      }
    });

    const transactionsRef = collection(db, "balance", newEmail, "transactions");
    cancelTransactionListener = onSnapshot(transactionsRef, (update) => {
      transactions.value = update.docs.map((doc) => doc.data() as Transaction);
      console.log("[db] transactions " + transactions.value.length);
    });
  }
});
