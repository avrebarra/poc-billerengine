import xid from "xid-js";

const DEFAULT_LOAN_AMOUNT = 5_000_000;
const DEFAULT_LOAN_DURATION_WEEK = 50;
const DEFAULT_INTEREST_RATE_PERCENTAGE = 10;
const DELIQUENCY_PAYMENT_SKIP_THRESHOLD = 2; // how many times payment have to be skipped to be categorized as late

interface Billable {
  ID: string;
  amount: number;
  principal: number;
  durWeek: number;
  createdAt: Date;
  dueAt: Date;
}

interface Payment {
  ID: string;
  billableID: string;
  amount: number;
  paidAt: Date;
  createdAt: Date;
}

const BillingEngine = () => {
  const billables: Billable[] = [];
  const payments: Payment[] = [];

  const makeBillable = (data: { bID: string; principal: number }) => {
    const { bID, principal } = data;

    // TODO: validate

    // validate existing bID
    if (billables.find((x) => x.ID == bID)) throw new Error("duplicate billable id");

    const timestamp = new Date();
    const amount = principal * ((DEFAULT_INTEREST_RATE_PERCENTAGE + 100) / 100);

    // determine due date
    const dueDate = new Date(timestamp);
    dueDate.setDate(dueDate.getDate() + DEFAULT_LOAN_DURATION_WEEK * 7);

    billables.push({
      ID: bID,
      principal: principal,
      amount: amount,
      durWeek: DEFAULT_LOAN_DURATION_WEEK,
      createdAt: timestamp,
      dueAt: dueDate,
    });
  };

  const getOutstanding = (bID: string): { principal: number; bill: number; paid: number; outstanding: number } => {
    const b = billables.find((x) => x.ID == bID);
    if (!b) throw new Error(`billable not found: id ${bID}`);

    // aggregate amount // TODO: heavy query, improve data efficiency
    const ps = payments.filter((x) => x.billableID == b.ID);
    const amountPaid = ps.reduce((prev, cur) => prev + cur.amount, 0);

    const out = { principal: b.principal, bill: b.amount, paid: amountPaid, outstanding: b.amount - amountPaid };

    return out;
  };

  const isDelinquent = (bID: string): { delinquency: boolean } => {
    const getWeeksSinceDate = (startDate: Date): number => {
      const millisecondsInWeek = 7 * 24 * 60 * 60 * 1000; // Number of milliseconds in a week
      const currentDate = new Date();
      const diffInMilliseconds = currentDate.getTime() - startDate.getTime();
      return Math.floor(diffInMilliseconds / millisecondsInWeek); // Calculate weeks from milliseconds
    };

    const b = billables.find((x) => x.ID == bID);
    if (!b) throw new Error(`billable not found: id ${bID}`);
    const weeklyBill = b.amount / b.durWeek;

    // aggregate amount // TODO: heavy query, improve data efficiency
    const ps = payments.filter((x) => x.billableID == b.ID);
    const amountPaid = ps.reduce((prev, cur) => prev + cur.amount, 0);

    const billableAgeWeek = getWeeksSinceDate(b.createdAt);
    const expectedAggrAmountPaid = (b.amount / b.durWeek) * billableAgeWeek;

    const out = { delinquency: expectedAggrAmountPaid - amountPaid > DELIQUENCY_PAYMENT_SKIP_THRESHOLD * weeklyBill };

    return out;
  };

  const makePayment = (bID: string, data: { amount: number; paidAt: Date }) => {
    const { amount, paidAt } = data;
    const timestamp = new Date();
    payments.push({ ID: xid.next(), billableID: bID, amount, paidAt, createdAt: timestamp });
  };

  return {
    makeBillable,
    getOutstanding,
    isDelinquent,
    makePayment,
  };
};

const be = BillingEngine();

const bID = xid.next();
be.makeBillable({ bID: bID, principal: DEFAULT_LOAN_AMOUNT });
be.makePayment(bID, {
  amount: (DEFAULT_LOAN_AMOUNT + (DEFAULT_LOAN_AMOUNT * DEFAULT_INTEREST_RATE_PERCENTAGE) / 100) / DEFAULT_LOAN_DURATION_WEEK,
  paidAt: new Date(),
});
console.log(be.isDelinquent(bID));
console.log(be.getOutstanding(bID));
