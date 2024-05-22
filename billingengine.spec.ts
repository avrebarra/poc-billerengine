import dayjs from "dayjs";
import * as billingengine from "./billingengine";

describe("BillingEngine", () => {
  test("works correctly", () => {
    const timestamp = dayjs("2022-01-01");
    const getCurrentDate = jest.fn((): Date => timestamp.toDate());
    const be = billingengine.BillingEngine({ getCurrentDate });

    // make billable
    const b1 = be.makeBillable({ bID: "1", principal: billingengine.DEFAULT_LOAN_AMOUNT });
    const b2 = be.makeBillable({ bID: "2", principal: billingengine.DEFAULT_LOAN_AMOUNT });
    const b3 = be.makeBillable({ bID: "3", principal: billingengine.DEFAULT_LOAN_AMOUNT });
    expect(b1).not.toBeUndefined();
    expect(b2).not.toBeUndefined();
    expect(b3).not.toBeUndefined();

    // assert stores
    expect(be.getBillables().length).toBe(3);
    expect(be.getPayments().length).toBe(0);

    // assert outstanding and states
    const outstanding = be.getOutstanding(b2.ID);
    const delinquency = be.isDelinquent(b2.ID);
    expect(outstanding.principal).toBe(billingengine.DEFAULT_LOAN_AMOUNT);
    expect(outstanding.bill).toBeGreaterThan(outstanding.principal);
    expect(delinquency.delinquency).toBeFalsy();

    // skip two week, check for delinquency: ok
    // skip one week, pay, skip one week, check for delinquency: ok
    console.log(timestamp.toDate());
    let curdate = dayjs(timestamp);

    curdate = timestamp.add(14, "day");
    console.log(curdate.toDate());
    getCurrentDate.mockImplementationOnce(() => curdate.toDate());
    be.makePayment(b2.ID, { amount: (billingengine.DEFAULT_LOAN_AMOUNT * 1.1) / 50, paidAt: curdate.toDate() });

    curdate = timestamp.add(21, "day");
    console.log(curdate.toDate());
    getCurrentDate.mockImplementationOnce(() => curdate.toDate());
    console.log(be.getOutstanding(b2.ID));
    console.log(be.isDelinquent(b2.ID));

    getCurrentDate.mockRestore();
  });
});
