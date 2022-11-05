import * as functions from "firebase-functions";
import { initializeApp } from "firebase-admin/app";
import { FieldValue, getFirestore } from "firebase-admin/firestore";
const Stripe = require("stripe");

const app = initializeApp();
export const firestore = getFirestore(app);

export const migratedb = functions
  .region("europe-west1")
  .https.onRequest(async (request, response) => {
    const balances = await firestore.collection("balance").get();
    for (const balance of balances.docs) {
      const balanceData = balance.data() as Record<string, string>;
      functions.logger.info(
        `Resetting paymentLink for ` + balance.id,
        balanceData
      );
      await addPaymentToBalance(balance.id);
    }

    const mailboxes = await firestore.collection("mailboxes").get();
    for (const mailbox of mailboxes.docs) {
      const emails = await firestore
        .collection("mailboxes")
        .doc(mailbox.id)
        .collection("emails")
        .get();
      for (const email of emails.docs) {
        functions.logger.info(
          `Resetting paymentLink for ${mailbox.id}/${email.id}`
        );
        const paymentLink = await createPaymentLink(
          {
            forUser: mailbox.id,
            forEmail: email.id,
          },
          `https://pay2mail.me/paid/${mailbox.id.split("@")[0]}/${email.id}`
        );
        if (!paymentLink) {
          throw new Error("No paymentlink gotten :(");
        }

        await firestore
          .collection("mailboxes")
          .doc(mailbox.id)
          .collection("emails")
          .doc(email.id)
          .update({ paymentLink });
      }
    }
    response.send("Done!");
  });

export const incomingMail = functions
  .region("europe-west1")
  .https.onRequest(async (request, response) => {
    const body = request.body as {
      data: string;
      date: string;
      id: string;
      recipient: string;
      sender: string;
      subject: string;
    };
    functions.logger.info(`got mail`, { body });

    // deduct from existing balance of the user
    const balanceRef = firestore.collection("balance").doc(body.sender);
    const balance = await balanceRef.get();
    if (balance.exists) {
      const balanceData = balance.data() as {
        balance: number;
      };
      if (balanceData.balance > 10) {
        await balanceRef.update({
          balance: FieldValue.increment(-10),
        });

        response.json({ paid: true });
        return;
      }
    }

    // insert in db
    const paymentLink = await createPaymentLink(
      {
        forUser: body.recipient,
        forEmail: body.id,
      },
      `https://pay2mail.me/paid/${body.recipient.split("@")[0]}/${body.id}`
    );
    if (!paymentLink) {
      throw new Error("No paymentlink gotten :(");
    }

    await firestore
      .collection("mailboxes")
      .doc(body.recipient)
      .collection("emails")
      .doc(body.id)
      .set({ ...body, paymentLink });

    response.json({ paid: false });
  });

export { stripeWebhook } from "./stripeWebhook";

/** user can build balance */
export const createUser = functions.auth.user().onCreate(async (user) => {
  const email = user.email;
  if (!email) {
    functions.logger.error("user created without email", { user });
    return;
  }
  await addPaymentToBalance(email);
});

const addPaymentToBalance = async (email: string) => {
  await firestore.runTransaction(async (t) => {
    const paymentLink = await createPaymentLink(
      { forUser: email },
      `https://pay2mail.me/`
    );
    if (!paymentLink) {
      throw new Error("No paymentlink gotten :(");
    }

    const ref = firestore.collection("balance").doc(email);
    const doc = await t.get(ref);
    functions.logger.info(`Adding paymentLink`);
    if (!doc.exists) {
      t.set(ref, {
        balance: 0,
        added: FieldValue.serverTimestamp(),
        paymentLink,
      });
    } else {
      t.update(ref, {
        paymentLink,
      });
    }
  });
};

// export const addPaymentLinkToEmail = functions.firestore
//   .document("mailboxes/{mailboxId}/emails/{emailId}")
//   .onCreate(async (doc, ctx) => {
//     if (doc.data().paymentLink) {
//       throw new Error(
//         `Email already has paymentLink: ${ctx.params.mailboxId}/${doc.id}`
//       );
//     }
//     const paymentLink = await createPaymentLink(
//       {
//         forUser: ctx.params.mailboxId,
//         forEmail: doc.id,
//       },
//       `https://pay2mail.me/paid/${ctx.params.mailboxId.split("@")[0]}/${doc.id}`
//     );
//     if (!paymentLink) {
//       throw new Error("No paymentlink gotten :(");
//     }
//     doc.ref.update({ paymentLink });
//   });

async function createPaymentLink(
  metadata: {
    forUser: string;
    forEmail?: string;
  },
  redirectUrl: string
) {
  const stripe = Stripe("sk_test_xJU4W5s1dOWiNjQ6oU8Nfs0S");
  // const productId = 'prod_MjvCZGLYUzcXZy';
  const priceId = "price_0M0RN72xzq6B1qO3DSWkTrn9";

  const paymentLink = await stripe.paymentLinks.create({
    line_items: [
      {
        price: priceId,
        quantity: 1,
      },
    ],
    metadata,
    after_completion: {
      redirect: {
        url: redirectUrl,
      },
      type: "redirect",
    },
  });

  functions.logger.info("Created paymentLink", paymentLink);
  return paymentLink.url;
}
