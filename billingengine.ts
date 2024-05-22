import xid from "xid-js";

export const DEFAULT_LOAN_AMOUNT = 5_000_000;
export const DEFAULT_LOAN_DURATION_WEEK = 50;
export const DEFAULT_INTEREST_RATE_PERCENTAGE = 10;
export const DELIQUENCY_PAYMENT_SKIP_THRESHOLD = 2; // how many times payment have to be skipped to be categorized as late

interface Billable {
  ID: string;
  amount: number;
  principal: number;
  durWeek: number;
  createdAt: Date;
  dueAt: Date;

  amountPaid: number;
}

interface Payment {
  ID: string;
  billableID: string;
  amount: number;
  paidAt: Date;
  createdAt: Date;
}

export const BillingEngine = (deps: { getCurrentDate: () => Date }) => {
  const billables: Billable[] = [];
  const payments: Payment[] = [];

  const makeBillable = (data: { bID: string; principal: number }): Billable => {
    const { bID, principal } = data;

    // TODO: validate

    // validate existing bID
    if (billables.find((x) => x.ID == bID)) throw new Error("duplicate billable id");

    const timestamp = deps.getCurrentDate();
    const amount = principal * ((DEFAULT_INTEREST_RATE_PERCENTAGE + 100) / 100);

    // determine due date
    const dueDate = new Date(timestamp);
    dueDate.setDate(dueDate.getDate() + DEFAULT_LOAN_DURATION_WEEK * 7);

    // make and push billable
    const b: Billable = {
      ID: bID,
      principal: principal,
      amount: amount,
      durWeek: DEFAULT_LOAN_DURATION_WEEK,
      createdAt: timestamp,
      dueAt: dueDate,

      amountPaid: 0,
    };

    billables.push(b);

    return b;
  };

  const getBillables = () => billables;
  const getPayments = () => payments;

  const getOutstanding = (bID: string) => {
    const b = billables.find((x) => x.ID == bID);
    if (!b) throw new Error(`billable not found: id ${bID}`);

    const out = { principal: b.principal, bill: b.amount, paid: b.amountPaid, outstanding: b.amount - b.amountPaid };

    return out;
  };

  const isDelinquent = (bID: string) => {
    const getWeeksSinceDate = (startDate: Date) => {
      const millisecondsInWeek = 7 * 24 * 60 * 60 * 1000; // Number of milliseconds in a week
      const currentDate = deps.getCurrentDate();
      const diffInMilliseconds = currentDate.getTime() - startDate.getTime();
      return Math.floor(diffInMilliseconds / millisecondsInWeek); // Calculate weeks from milliseconds
    };

    const b = billables.find((x) => x.ID == bID);
    if (!b) throw new Error(`billable not found: id ${bID}`);
    const weeklyBill = b.amount / b.durWeek;
    const amountPaid = b.amountPaid;

    const billableAgeWeek = getWeeksSinceDate(b.createdAt);
    const expectedAggrAmountPaid = (b.amount / b.durWeek) * billableAgeWeek;

    const out = { delinquency: expectedAggrAmountPaid - amountPaid >= DELIQUENCY_PAYMENT_SKIP_THRESHOLD * weeklyBill };

    return out;
  };

  const makePayment = (bID: string, data: { amount: number; paidAt: Date }) => {
    const { amount, paidAt } = data;
    const timestamp = deps.getCurrentDate();

    const b = billables.find((x) => x.ID == bID);
    if (!b) throw new Error(`billable not found: id ${bID}`);

    // TODO: wrap in transaction
    payments.push({ ID: xid.next(), billableID: bID, amount, paidAt, createdAt: timestamp });
    b.amountPaid = b.amountPaid + amount;
  };

  return {
    getBillables,
    getPayments,
    makeBillable,
    getOutstanding,
    isDelinquent,
    makePayment,
  };
};
